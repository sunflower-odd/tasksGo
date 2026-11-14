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
