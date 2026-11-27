package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"ledger/internal/domain"
)

type FinanceService interface {
	SetBudget(ctx context.Context, b domain.Budget) error
	ListBudgets(ctx context.Context) ([]domain.Budget, error)
	AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error)
	ListTransactions(ctx context.Context) ([]domain.Transaction, error)
	GetReportSummary(ctx context.Context, from, to string) (domain.Report, error)
}

type financeService struct {
	budgets      domain.BudgetRepository
	expenses     domain.ExpenseRepository
	cache        domain.ReportCache
	cacheBudgets domain.BudgetCache
}

func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository, reportCache domain.ReportCache, budgetsCache domain.BudgetCache) FinanceService {
	return &financeService{budgets: budgets, expenses: expenses, cache: reportCache, cacheBudgets: budgetsCache}
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

	if err := ctx.Err(); err != nil {
		return domain.Report{}, err
	}
	if s.cache != nil {
		if report, found := s.cache.Get(ctx, from, to); found {
			if err := ctx.Err(); err != nil {
				return domain.Report{}, err
			}
			return report, nil
		}
	}

	categories, err := s.expenses.GetUniqueCategories(ctx)
	if err != nil {
		return domain.Report{}, fmt.Errorf("create reports: %w", err)
	}

	type categoryResult struct {
		total float64
		err   error
	}
	results := make([]categoryResult, len(categories))
	// Один heartbeat на весь расчёт; каналы сигнализируют об окончании расчёта и выходе фоновой горутины.
	calculationDone := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(500 * time.Millisecond)
		// Фиксируем остановку тикера до уведомления о завершении heartbeat.
		defer func() {
			ticker.Stop()
			log.Print("report heartbeat stopped")
		}()
		for {
			select {
			case <-ctx.Done():
				log.Printf("report heartbeat canceled via ctx.Done(): %v", ctx.Err())
				return
			case <-calculationDone:
				return
			case <-ticker.C:
				log.Print("report calculation in progress")
			}
		}
	}()
	var wg sync.WaitGroup
	for i, category := range categories {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(index int, category string) {
			defer wg.Done()
			if err := ctx.Err(); err != nil {
				results[index].err = err
				return
			}
			results[index].total, results[index].err = s.expenses.TotalByCategory(ctx, category, from, to)
		}(i, category)
	}
	wg.Wait()
	// Останавливаем heartbeat и ждём его выхода, прежде чем обрабатывать результаты или возвращать ошибку.
	close(calculationDone)
	<-heartbeatDone
	if err := ctx.Err(); err != nil {
		// Wait завершён: все рабочие горутины уже вышли
		log.Printf("report calculation canceled via ctx.Done(): %v; all workers finished", err)
		return domain.Report{}, err
	}
	totals := make(map[string]float64, len(categories))
	for i, result := range results {
		if result.err != nil {
			return domain.Report{}, fmt.Errorf("create report for category %q: %w", categories[i], result.err)
		}
		totals[categories[i]] = result.total
	}

	report := domain.Report{
		StartDate: from,
		EndDate:   to,
		Totals:    totals,
	}

	if s.cache != nil {
		s.cache.Set(ctx, report)
	}
	if err := ctx.Err(); err != nil {
		return domain.Report{}, err
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
	if s.cacheBudgets != nil {
		if budgets, found := s.cacheBudgets.GetBudgets(ctx); found {
			if err := ctx.Err(); err != nil {
				return []domain.Budget{}, err
			}
			return budgets, nil
		}
	}

	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, err
	}

	if s.cacheBudgets != nil {
		s.cacheBudgets.SetBudgets(ctx, budgets)
	}

	return budgets, nil
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
