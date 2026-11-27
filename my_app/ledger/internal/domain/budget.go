package domain

import (
	"context"
	"errors"
)

type Budget struct {
	Category string
	Limit    float64
	Period   string
}

type BudgetRepository interface {
	Upsert(ctx context.Context, b Budget) error
	GetByCategory(ctx context.Context, category string) (Budget, error)
	List(ctx context.Context) ([]Budget, error)
}

type BudgetCache interface {
	GetBudgets(ctx context.Context) ([]Budget, bool) // bool = нашли в кеше
	SetBudgets(ctx context.Context, r []Budget)
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

var (
	ErrBudgetNotFound = errors.New("budget not found")
	ErrBudgetExceeded = errors.New("budget exceeded")
)
