package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	err := SetBudget(Budget{Category: "еда", Limit: 5000})
	assert.NoError(t, err)

	// в пределах лимита
	err = AddTransaction(Transaction{Amount: 3000, Category: "еда"})
	assert.NoError(t, err)

	before := len(ListTransactions())

	// 3000 + 2500 > 5000 — превышение
	err = AddTransaction(Transaction{Amount: 2500, Category: "еда"})
	assert.ErrorIs(t, err, ErrBudgetExceeded)

	assert.Len(t, ListTransactions(), before)
}
