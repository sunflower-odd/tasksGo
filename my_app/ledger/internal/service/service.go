package service

import (
	"context"
	"errors"
	"fmt"

	"ledger/internal/domain"
	"time"
)

type FinanceService interface {
	SetBudget(ctx context.Context, b domain.Budget) error
	ListBudgets(ctx context.Context) ([]domain.Budget, error)
	AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error)
	ListTransactions(ctx context.Context) ([]domain.Transaction, error)
	GetReportSummary(ctx context.Context, from, to string) (domain.Report, error)
}

type financeService struct {
	budgets  domain.BudgetRepository
	expenses domain.ExpenseRepository
	cache    domain.ReportCache
}

func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository, reportCache domain.ReportCache) FinanceService {
	return &financeService{budgets: budgets, expenses: expenses, cache: reportCache}
}

func (s *financeService) GetReportSummary(ctx context.Context, from string, to string) (domain.Report, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return domain.Report{}, fmt.Errorf("%w: start date must be YYYY-MM-DD", domain.ErrInvalidPeriod)
	}

	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return domain.Report{}, fmt.Errorf("%w: end date must be YYYY-MM-DD", domain.ErrInvalidPeriod)
	}

	if fromDate.After(toDate) {
		return domain.Report{}, fmt.Errorf("%w: end date earlier than start date of the report", domain.ErrInvalidPeriod)
	}

	if s.cache != nil {
		if report, found := s.cache.Get(ctx, from, to); found {
			return report, nil
		}
	}

	totals, err := s.expenses.TotalsByCategory(ctx, from, to)
	if err != nil {
		return domain.Report{}, fmt.Errorf("create reports: %w", err)
	}

	report := domain.Report{
		StartDate: from,
		EndDate:   to,
		Totals:    totals,
	}

	if s.cache != nil {
		s.cache.Set(ctx, report)
	}

	return report, nil

}

func (s *financeService) SetBudget(ctx context.Context, b domain.Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}
	return s.budgets.Upsert(ctx, b)
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
