package pg

import (
	"context"
	"database/sql"
	"fmt"
	"ledger/internal/domain"
)

type ExpenseRepo struct {
	db *sql.DB
}

func NewExpenseRepo(db *sql.DB) *ExpenseRepo {
	return &ExpenseRepo{db: db}
}

func (r *ExpenseRepo) List(ctx context.Context) ([]domain.Transaction, error) {

	result := []domain.Transaction{}

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, amount, category, COALESCE(description, ''), date::text FROM expenses ORDER BY date DESC, id DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &tx.Date); err != nil {
			return nil, fmt.Errorf("scan expenses: %w", err)
		}
		result = append(result, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}

	return result, nil
}

func (r *ExpenseRepo) Add(ctx context.Context, tx domain.Transaction) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO expenses(amount, category, description, date) VALUES($1,$2,$3,$4) RETURNING id",
		tx.Amount, tx.Category, tx.Description, tx.Date,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert transaction: %w", err)
	}
	return id, nil
}

func (r *ExpenseRepo) SumByCategory(ctx context.Context, category string) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(amount),0) FROM expenses WHERE category=$1", category,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum expenses for category %q: %w", category, err)
	}
	return total, nil
}

func (r *ExpenseRepo) TotalsByCategory(ctx context.Context, from, to string) (map[string]float64, error) {
	totals := make(map[string]float64)
	rows, err := r.db.QueryContext(ctx,
		"SELECT category, sum(amount) as Amount FROM expenses WHERE date>=$1 AND date<=$2 GROUP BY category ORDER BY category", from, to,
	)

	if err != nil {
		return nil, fmt.Errorf("expenses for the period %q–%q: %w", from, to, err)
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var amount float64

		if err := rows.Scan(&category, &amount); err != nil {
			return nil, fmt.Errorf("scan category totals: %w", err)
		}
		totals[category] = amount
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read category totals: %w", err)
	}

	return totals, nil
}

func (r *ExpenseRepo) GetUniqueCategories(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT DISTINCT category FROM expenses ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("get unique categories: %w", err)
	}
	defer rows.Close()
	categories := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get unique categories: %w", err)
	}
	return categories, nil
}

func (r *ExpenseRepo) TotalByCategory(ctx context.Context, category string, from, to string) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(amount),0) FROM expenses WHERE category=$1 AND date>=$2 AND date<=$3", category, from, to,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum expenses for category %q: %w", category, err)
	}
	return total, nil
}
