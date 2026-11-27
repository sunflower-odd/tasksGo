package app

import (
	"context"
	"errors"
	"fmt"
	"ledger/internal/cache"
	"ledger/internal/db"
	"ledger/internal/domain"
	"ledger/internal/repository/pg"
	"ledger/internal/service"
)

func New(ctx context.Context) (service.FinanceService, func() error, error) {
	conn, err := db.Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("init ledger: %w", err)
	}

	var reportCache domain.ReportCache
	var budgetCache domain.BudgetCache
	redisConn, err := cache.Connect()
	if err != nil {
		fmt.Printf("redis unavailable, working without cache: %v", err)
	} else {
		reportCache = cache.NewReportCache(redisConn)
		budgetCache = cache.NewBudgetCache(redisConn)
	}

	budgetRepo := pg.NewBudgetRepo(conn)
	expenseRepo := pg.NewExpenseRepo(conn)
	svc := service.New(budgetRepo, expenseRepo, reportCache, budgetCache)
	closeFn := func() error {
		dbErr := conn.Close()
		var redisErr error
		if redisConn != nil {
			redisErr = redisConn.Close()
		}
		return errors.Join(dbErr, redisErr)
	}
	return svc, closeFn, nil
}
