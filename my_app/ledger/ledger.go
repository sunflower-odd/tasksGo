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

	"github.com/redis/go-redis/v9"
)

// {"from":"2026-10-01","to":"2026-10-31","totals":{"food":3000}}
type Report struct {
	StartDate string             `json:"from"`
	EndDate   string             `json:"to"`
	Totals    map[string]float64 `json:"totals"`
}

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

var transactions []Transaction

type Budget struct {
	Category string
	Limit    float64
	Period   string
}

var budgets = make(map[string]Budget)
var ErrBudgetExceeded = errors.New("budget exceeded")

var ErrInvalidPeriod = errors.New("invalid report period")

type Validatable interface {
	Validate() error
}

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

	conn, err := db.Connect()
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

func Reset() {
	transactions = []Transaction{}
	budgets = make(map[string]Budget)
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("Amount must be greater than 0!")
	}

	if tx.Category == "" {
		return errors.New("Category should not be empty!")
	}

	return nil

}

func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New("Limit must be greater than 0!")
	}

	if b.Category == "" {
		return errors.New("Category should not be empty!")
	}

	return nil

}

func sumTransactions(category string) float64 {
	var total float64

	for _, tx := range transactions {
		if tx.Category == category {
			total += tx.Amount
		}
	}

	return total
}

// addTransaction проверяет лимит бюджета и сохраняет транзакцию в базу, возвращает её id
func addTransaction(tx Transaction) (int, error) {

	if err := tx.Validate(); err != nil {
		return 0, err
	}

	if err := ensureConn(); err != nil {
		return 0, err
	}

	var limit float64
	err := myConn.QueryRow("SELECT limit_amount FROM budgets WHERE category=$1", tx.Category).Scan(&limit)

	if errors.Is(err, sql.ErrNoRows) {
		// бюджета для категории нет — добавляем без проверки лимита
	} else if err != nil {
		return 0, fmt.Errorf("get budget for category %q: %w", tx.Category, err)
	} else {
		var spent float64
		errSpent := myConn.QueryRow("SELECT COALESCE(SUM(amount),0) FROM expenses WHERE category=$1", tx.Category).Scan(&spent)
		if errSpent != nil {
			return 0, fmt.Errorf("get spent amount for category %q: %w", tx.Category, errSpent)
		}
		if spent+tx.Amount > limit {
			return 0, ErrBudgetExceeded
		}
	}

	var id int
	err = myConn.QueryRow(
		"INSERT INTO expenses(amount, category, description, date) VALUES($1,$2,$3,$4) RETURNING id",
		tx.Amount, tx.Category, tx.Description, tx.Date,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert transaction: %w", err)
	}

	return id, nil
}

func AddTransaction(tx Transaction) error {
	_, err := addTransaction(tx)
	return err
}

// добавляет транзакцию и возвращает её с id, выданным базой
func CreateTransaction(tx Transaction) (Transaction, error) {
	id, err := addTransaction(tx)
	if err != nil {
		return Transaction{}, err
	}
	tx.ID = id
	return tx, nil
}

func ListTransactions() ([]Transaction, error) {
	if err := ensureConn(); err != nil {
		return nil, err
	}

	result := []Transaction{}

	rows, err := myConn.Query(
		"SELECT id, amount, category, COALESCE(description, ''), date::text FROM expenses ORDER BY date DESC, id DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &tx.Date); err != nil {
			return nil, fmt.Errorf("scan expenses: %w", err)
		}
		result = append(result, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}

	return result, nil
}

func ListBudgets() ([]Budget, error) {
	if err := ensureConn(); err != nil {
		return nil, err
	}

	result := []Budget{}

	rows, err := myConn.Query("SELECT category, limit_amount FROM budgets ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.Category, &b.Limit); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}

	return result, nil
}

func SetBudget(b Budget) error {

	if err := b.Validate(); err != nil {
		return err
	}

	if err := ensureConn(); err != nil {
		return err
	}

	_, err := myConn.Exec(
		"INSERT INTO budgets(category, limit_amount ) VALUES($1,$2) ON CONFLICT(category) DO UPDATE SET limit_amount =EXCLUDED.limit_amount",
		b.Category, b.Limit,
	)
	if err != nil {
		return fmt.Errorf("set budget for category %q: %w", b.Category, err)
	}

	return nil
}

func LoadBudgets(r io.Reader) error {
	var loadedBudgets []Budget
	err := json.NewDecoder(r).Decode(&loadedBudgets)

	if err != nil {
		return fmt.Errorf("budget file can not be parsed: %w", err)
	}

	for _, budget := range loadedBudgets {
		if err := SetBudget(budget); err != nil {
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

func CheckValid(v Validatable) error {
	err := v.Validate()
	if err != nil {
		return fmt.Errorf("Object could not be validated: %w", err)
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
