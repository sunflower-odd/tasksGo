package domain

import (
	"context"
	"errors"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

type ExpenseRepository interface {
	List(ctx context.Context) ([]Transaction, error)
	Add(ctx context.Context, tx Transaction) (int, error)
	SumByCategory(ctx context.Context, category string) (float64, error)
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("amount must be greater than 0!")
	}

	if tx.Category == "" {
		return errors.New("category should not be empty")
	}

	return nil

}
