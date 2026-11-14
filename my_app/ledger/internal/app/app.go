package app

import (
	"context"
	"fmt"
	"ledger/internal/db"
	"ledger/internal/repository/pg"
	"ledger/internal/service"
)

func New(ctx context.Context) (service.FinanceService, func() error, error) {
	conn, err := db.Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("init ledger: %w", err)
	}

	budgetRepo := pg.NewBudgetRepo(conn)
	expenseRepo := pg.NewExpenseRepo(conn)
	svc := service.New(budgetRepo, expenseRepo)
	return svc, conn.Close, nil
}
