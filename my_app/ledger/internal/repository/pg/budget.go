package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"ledger/internal/domain"
)

type BudgetRepo struct {
	db *sql.DB
}

func NewBudgetRepo(db *sql.DB) *BudgetRepo {
	return &BudgetRepo{db: db}
}

func (r *BudgetRepo) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	b := domain.Budget{Category: category}
	err := r.db.QueryRowContext(ctx,
		"SELECT limit_amount FROM budgets WHERE category=$1", category,
	).Scan(&b.Limit)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Budget{}, domain.ErrBudgetNotFound
	}
	if err != nil {
		return domain.Budget{}, fmt.Errorf("get budget %q: %w", category, err)
	}
	return b, nil
}

func (r *BudgetRepo) List(ctx context.Context) ([]domain.Budget, error) {

	result := []domain.Budget{}

	rows, err := r.db.QueryContext(ctx, "SELECT category, limit_amount FROM budgets ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b domain.Budget
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

func (r *BudgetRepo) Upsert(ctx context.Context, b domain.Budget) error {

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO budgets(category, limit_amount ) VALUES($1,$2) ON CONFLICT(category) DO UPDATE SET limit_amount =EXCLUDED.limit_amount",
		b.Category, b.Limit,
	)
	if err != nil {
		return fmt.Errorf("set budget for category %q: %w", b.Category, err)
	}

	return nil
}
