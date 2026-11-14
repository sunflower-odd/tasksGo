package service

import (
	"context"
	"errors"
	"ledger/internal/domain"
)

type FinanceService interface {
	SetBudget(ctx context.Context, b domain.Budget) error
	ListBudgets(ctx context.Context) ([]domain.Budget, error)
	AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error)
	ListTransactions(ctx context.Context) ([]domain.Transaction, error)
}

type financeService struct {
	budgets  domain.BudgetRepository
	expenses domain.ExpenseRepository
}

func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository) FinanceService {
	return &financeService{budgets: budgets, expenses: expenses}
}

func (s *financeService) SetBudget(ctx context.Context, b domain.Budget) error {
	if err := b.Validate(); err != nil { // ← логика осталась та же
		return err
	}
	return s.budgets.Upsert(ctx, b) // ← вместо SQL — вызов репозитория
}

func (s *financeService) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	return s.budgets.List(ctx)
}

func (s *financeService) ListTransactions(ctx context.Context) ([]domain.Transaction, error) {
	return s.expenses.List(ctx)
}

func (s *financeService) AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if err := tx.Validate(); err != nil {
		return domain.Transaction{}, err
	}

	budget, err := s.budgets.GetByCategory(ctx, tx.Category)

	if errors.Is(err, domain.ErrBudgetNotFound) {
		// бюджета для категории нет — добавляем без проверки лимита
	} else if err != nil {
		return domain.Transaction{}, err
	} else {
		spent, err := s.expenses.SumByCategory(ctx, tx.Category)
		if err != nil {
			return domain.Transaction{}, err
		}
		if spent+tx.Amount > budget.Limit {
			return domain.Transaction{}, domain.ErrBudgetExceeded
		}
	}

	id, err := s.expenses.Add(ctx, tx)
	if err != nil {
		return domain.Transaction{}, err
	}
	tx.ID = id
	return tx, nil
}
