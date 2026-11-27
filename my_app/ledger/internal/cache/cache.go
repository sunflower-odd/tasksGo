package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"ledger/internal/domain"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Connect() (*redis.Client, error) {
	addr := getEnv("REDIS_ADDR", "localhost:6379")
	password := os.Getenv("REDIS_PASSWORD")

	dbNum, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       dbNum,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close() // освобождаем клиент при неудачном подключении
		return nil, fmt.Errorf("unable to connect to redis at %s: %w", addr, err)
	}

	fmt.Println("Connected to Redis")

	return client, nil
}

type ReportCache struct {
	client *redis.Client
}

const reportCacheTTL = 30 * time.Second

type BudgetCache struct {
	client *redis.Client
}

const budgetCacheKey = "budgets:all"
const budgetCacheTTL = 30 * time.Second

func NewReportCache(client *redis.Client) *ReportCache {
	return &ReportCache{client: client}
}

func NewBudgetCache(client *redis.Client) *BudgetCache {
	return &BudgetCache{client: client}
}

func (c *BudgetCache) GetBudgets(ctx context.Context) ([]domain.Budget, bool) {
	if c == nil || c.client == nil {
		fmt.Println("cache: Redis is not initialized, get data from db")
		return nil, false
	}

	data, err := c.client.Get(ctx, budgetCacheKey).Bytes()
	if errors.Is(err, redis.Nil) {
		fmt.Println("cache miss:", budgetCacheKey)
		return nil, false
	}

	if err != nil {
		fmt.Println("cache: error getting data from Redis, get data from db:", err)
		return nil, false
	}

	var cached []domain.Budget
	if err := json.Unmarshal(data, &cached); err != nil {
		fmt.Println("cache: could not parse cached budgets, get data from db:", budgetCacheKey, err)
		return nil, false
	}

	fmt.Println("cache hit:", budgetCacheKey)
	return cached, true
}

func (c *ReportCache) Get(ctx context.Context, from, to string) (domain.Report, bool) {
	if c == nil || c.client == nil {
		fmt.Println("cache: Redis is not initialized, get data from db")
		return domain.Report{}, false
	}

	cacheKey := fmt.Sprintf("report:summary:%s:%s", from, to)

	data, err := c.client.Get(ctx, cacheKey).Bytes()
	if errors.Is(err, redis.Nil) {
		fmt.Println("cache miss:", cacheKey)
		return domain.Report{}, false
	}

	if err != nil {
		fmt.Println("cache: error getting data from Redis, get data from db:", err)
		return domain.Report{}, false
	}

	var cached domain.Report
	if err := json.Unmarshal(data, &cached); err != nil {
		fmt.Println("cache: could not parse cached report, get data from db:", cacheKey, err)
		return domain.Report{}, false
	}

	fmt.Println("cache hit:", cacheKey)
	return cached, true
}

func (c *ReportCache) Set(ctx context.Context, report domain.Report) {
	if c == nil || c.client == nil {
		fmt.Println("cache: Redis is not initialized, skip cache write")
		return
	}

	cacheKey := fmt.Sprintf("report:summary:%s:%s", report.StartDate, report.EndDate)
	data, err := json.Marshal(report)
	if err != nil {
		fmt.Println("cache: could not serialize report:", err)
		return
	}
	if err := c.client.Set(ctx, cacheKey, data, reportCacheTTL).Err(); err != nil {
		fmt.Println("cache: could not write to Redis:", err)
		return
	}
	fmt.Println("cache set:", cacheKey, "TTL", reportCacheTTL)
}

func (c *BudgetCache) SetBudgets(ctx context.Context, budgets []domain.Budget) {
	if c == nil || c.client == nil {
		fmt.Println("cache: Redis is not initialized, skip cache write")
		return
	}

	data, err := json.Marshal(budgets)
	if err != nil {
		fmt.Println("cache: could not serialize budgets:", err)
		return
	}

	if err := c.client.Set(ctx, budgetCacheKey, data, budgetCacheTTL).Err(); err != nil {
		fmt.Println("cache: could not write to Redis:", err)
		return
	}
	fmt.Println("cache set:", budgetCacheKey, "TTL", budgetCacheTTL)
}
