package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"ledger/internal/cache"
	"ledger/internal/db"
	"ledger/internal/domain"

	"github.com/redis/go-redis/v9"
)

// {"from":"2026-10-01","to":"2026-10-31","totals":{"food":3000}}
type Report struct {
	StartDate string             `json:"from"`
	EndDate   string             `json:"to"`
	Totals    map[string]float64 `json:"totals"`
}

type Transaction = domain.Transaction
type Budget = domain.Budget

var ErrBudgetExceeded = domain.ErrBudgetExceeded

var ErrInvalidPeriod = errors.New("invalid report period")

type Validatable = domain.Validatable

var CheckValid = domain.CheckValid

var myRedisConn *redis.Client

var myConn *sql.DB

func InitDB(conn *sql.DB) {
	myConn = conn
}

func InitCache(conn *redis.Client) {
	myRedisConn = conn
}

const reportCacheTTL = 30 * time.Second

// connMu защищает ленивую инициализацию: HTTP-запросы в gateway идут параллельно.
var connMu sync.Mutex

// ensureConn подключает базу (и, по возможности, Redis) при первом обращении, если это ещё не сделано через InitDB/InitCache
func ensureConn() error {
	connMu.Lock()
	defer connMu.Unlock()

	if myConn != nil {
		return nil
	}

	conn, err := db.Connect(context.Background())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	myConn = conn
	fmt.Println("Connected to database")

	if myRedisConn == nil {
		rdb, err := cache.Connect()
		if err != nil {
			fmt.Println("cache: Redis is unavailable, working without cache:", err)
		} else {
			myRedisConn = rdb
			fmt.Println("Connected to Redis")
		}
	}

	return nil
}

func LoadBudgets(ctx context.Context, svc Service, r io.Reader) error {
	var loadedBudgets []Budget
	err := json.NewDecoder(r).Decode(&loadedBudgets)

	if err != nil {
		return fmt.Errorf("budget file can not be parsed: %w", err)
	}

	for _, budget := range loadedBudgets {
		if err := svc.SetBudget(ctx, budget); err != nil {
			return fmt.Errorf("invalid budget for category %q: %w", budget.Category, err)
		}
	}
	return nil
}

// DeleteCategory удаляет бюджет и все траты категории (используется тестами для очистки).
func DeleteCategory(category string) error {
	if err := ensureConn(); err != nil {
		return err
	}
	if _, err := myConn.Exec("DELETE FROM expenses WHERE category=$1", category); err != nil {
		return fmt.Errorf("delete expenses for category %q: %w", category, err)
	}
	if _, err := myConn.Exec("DELETE FROM budgets WHERE category=$1", category); err != nil {
		return fmt.Errorf("delete budget for category %q: %w", category, err)
	}
	return nil
}

func GetReportSummary(ctx context.Context, from string, to string) (Report, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return Report{}, fmt.Errorf("%w: start date must be YYYY-MM-DD", ErrInvalidPeriod)
	}

	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return Report{}, fmt.Errorf("%w: end date must be YYYY-MM-DD", ErrInvalidPeriod)
	}

	if fromDate.After(toDate) {
		return Report{}, fmt.Errorf("%w: end date earlier than start date of the report", ErrInvalidPeriod)
	}

	if err := ensureConn(); err != nil {
		return Report{}, err
	}

	cacheKey := fmt.Sprintf("report:summary:%s:%s", from, to)
	if myRedisConn != nil {
		data, err := myRedisConn.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var cached Report
			if err := json.Unmarshal(data, &cached); err != nil {
				fmt.Println("cache: could not parse cached report, get data from db:", cacheKey, err)
			} else {
				fmt.Println("cache hit:", cacheKey)
				return cached, nil
			}
		} else if errors.Is(err, redis.Nil) {
			fmt.Println("cache miss:", cacheKey)
		} else {
			fmt.Println("cache: error getting data from Redis, get data from db:", err)
		}
	}

	rows, err := myConn.QueryContext(ctx, "SELECT category, sum(amount) FROM expenses where date>=$1 and date<=$2 GROUP BY category ORDER BY category", from, to)
	if err != nil {
		return Report{}, fmt.Errorf("create reports: %w", err)
	}
	defer rows.Close()

	totals := make(map[string]float64)

	for rows.Next() {
		var category string
		var amount float64
		err = rows.Scan(&category, &amount)
		if err != nil {
			return Report{}, fmt.Errorf("could not read from database: %w", err)
		}
		totals[category] = amount
	}

	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("could not read from database: %w", err)
	}

	result := Report{
		StartDate: from,
		EndDate:   to,
		Totals:    totals,
	}

	if myRedisConn != nil {
		data, err := json.Marshal(result)
		if err != nil {
			fmt.Println("cache: could not serialize report:", err)
		} else if err := myRedisConn.Set(ctx, cacheKey, data, reportCacheTTL).Err(); err != nil {
			fmt.Println("cache: could not write to Redis:", err)
		} else {
			fmt.Println("cache set:", cacheKey, "TTL", reportCacheTTL)
		}
	}

	return result, nil
}
