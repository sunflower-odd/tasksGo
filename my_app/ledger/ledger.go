package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"ledger/internal/db"
	"ledger/internal/domain"
)

type Transaction = domain.Transaction
type Budget = domain.Budget
type Report = domain.Report

var ErrBudgetExceeded = domain.ErrBudgetExceeded
var ErrInvalidPeriod = domain.ErrInvalidPeriod

type Validatable = domain.Validatable

var CheckValid = domain.CheckValid

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

	myConn, err := db.Connect(context.Background())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer myConn.Close()

	if _, err := myConn.Exec(
		"DELETE FROM expenses WHERE category=$1", category,
	); err != nil {
		return fmt.Errorf("delete expenses for category %q: %w", category, err)
	}

	if _, err := myConn.Exec("DELETE FROM budgets WHERE category=$1", category); err != nil {
		return fmt.Errorf("delete budget for category %q: %w", category, err)
	}
	return nil
}
