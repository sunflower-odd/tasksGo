package ledger

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCategory подключается к БД (или пропускает тест, если БД недоступна)
// и возвращает уникальную категорию, которая удаляется после теста.
func testCategory(t *testing.T) string {
	t.Helper()
	if err := ensureConn(); err != nil {
		t.Skipf("database is not available: %v", err)
	}
	category := fmt.Sprintf("test-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = DeleteCategory(category) })
	return category
}

func countByCategory(t *testing.T, category string) int {
	t.Helper()
	list, err := ListTransactions()
	require.NoError(t, err)
	n := 0
	for _, tx := range list {
		if tx.Category == category {
			n++
		}
	}
	return n
}

func TestTransactionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tx      Transaction
		wantErr bool
	}{
		{
			name: "valid transaction",
			tx: Transaction{
				Amount:      100,
				Category:    "food",
				Description: "Lunch",
			},
			wantErr: false,
		},
		{
			name: "zero amount",
			tx: Transaction{
				Amount:   0,
				Category: "food",
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			tx: Transaction{
				Amount:   -10,
				Category: "food",
			},
			wantErr: true,
		},
		{
			name: "empty category",
			tx: Transaction{
				Amount:   100,
				Category: "",
			},
			wantErr: true,
		},
		{
			name: "empty description",
			tx: Transaction{
				Amount:      100,
				Category:    "food",
				Description: "",
			},
			wantErr: false,
		},
		{
			name: "zero amount and empty category",
			tx: Transaction{
				Amount:   0,
				Category: "",
			},
			wantErr: true,
		},
		{
			name: "valid fractional amount",
			tx: Transaction{
				Amount:   10.5,
				Category: "food",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.tx.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		b       Budget
		wantErr bool
	}{
		{
			name: "valid budget",
			b: Budget{
				Limit:    100,
				Category: "food",
			},
			wantErr: false,
		},
		{
			name: "empty category",
			b: Budget{
				Limit:    100,
				Category: "",
			},
			wantErr: true,
		},
		{
			name: "negative budget",
			b: Budget{
				Limit:    -100,
				Category: "electronics",
			},
			wantErr: true,
		},
		{
			name: "zero budget",
			b: Budget{
				Limit:    0,
				Category: "electronics",
			},
			wantErr: true,
		},
		{
			name: "negative budget and empty category",
			b: Budget{
				Limit:    -10,
				Category: "",
			},
			wantErr: true,
		},
		{
			name: "valid fractional amount",
			b: Budget{
				Limit:    10.5,
				Category: "food",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.b.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func TestBudgetLimit(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	category := testCategory(t)

	err := SetBudget(Budget{Category: category, Limit: 5000})
	assert.NoError(t, err)

	// в пределах лимита
	err = AddTransaction(Transaction{Amount: 3000, Category: category, Date: "2026-10-30"})
	assert.NoError(t, err)
	assert.Equal(t, 1, countByCategory(t, category))

	// 3000 + 2500 > 5000 — превышение, запись не добавляется
	err = AddTransaction(Transaction{Amount: 2500, Category: category, Date: "2026-10-30"})
	assert.ErrorIs(t, err, ErrBudgetExceeded)
	assert.Equal(t, 1, countByCategory(t, category))
}
