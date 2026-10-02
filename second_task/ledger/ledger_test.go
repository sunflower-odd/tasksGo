package main

import (
	"strings"
	"testing"
)

func resetState() {
	transactions = []Transaction{}
	budgets = make(map[string]Budget)
}

func TestAddTransaction(t *testing.T) {
	resetState()

	err := AddTransaction(Transaction{
		ID:          1,
		Amount:      100,
		Category:    "food",
		Description: "groceries",
		Date:        "2026-10-01",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(transactions))
	}
}

func TestAddTransactionWithinBudget(t *testing.T) {
	resetState()
	SetBudget(Budget{Category: "food", Limit: 100, Period: "2026/10"})

	// 60 + 40 = 100, edge case, ровно лимит
	for i, amount := range []float64{60, 40} {
		err := AddTransaction(Transaction{ID: i + 1, Amount: amount, Category: "food"})
		if err != nil {
			t.Fatalf("transaction %d: expected no error, got %v", i+1, err)
		}
	}
	if len(transactions) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(transactions))
	}
}

func TestAddTransactionExceedsBudget(t *testing.T) {
	resetState()
	SetBudget(Budget{Category: "food", Limit: 100, Period: "2026/10"})

	if err := AddTransaction(Transaction{ID: 1, Amount: 80, Category: "food"}); err != nil {
		t.Fatalf("first transaction: expected no error, got %v", err)
	}

	// 80 + 30 = 110 > 100: больше лимита, транзакция не должна сохраниться.
	err := AddTransaction(Transaction{ID: 2, Amount: 30, Category: "food"})
	if err == nil {
		t.Fatal("expected budget exceeded error, got nil")
	}

	list := ListTransactions()
	if len(list) != 1 {
		t.Fatalf("expected 1 transaction after rejection, got %d", len(list))
	}
	if list[0].ID != 1 {
		t.Errorf("rejected transaction was stored: %+v", list)
	}
}

func TestAddTransactionZeroAmount(t *testing.T) {
	resetState()

	if err := AddTransaction(Transaction{ID: 1, Amount: 0, Category: "food"}); err == nil {
		t.Error("expected error for zero amount, got nil")
	}
	if len(transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(transactions))
	}
}

func TestAddTransactionNoBudgetForCategory(t *testing.T) {
	resetState()

	// Для категории без бюджета лимита нет.
	if err := AddTransaction(Transaction{ID: 1, Amount: 1000000, Category: "asset"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSetBudgetUpdatesLimit(t *testing.T) {
	resetState()

	SetBudget(Budget{Category: "food", Limit: 100, Period: "2026/10"})
	SetBudget(Budget{Category: "food", Limit: 500, Period: "2026/10"})

	if len(budgets) != 1 {
		t.Errorf("expected 1 budget, got %d", len(budgets))
	}
	if budgets["food"].Limit != 500 {
		t.Errorf("expected limit 500, got %v", budgets["food"].Limit)
	}
}

func TestLoadBudgets(t *testing.T) {
	resetState()

	input := `[{"Category": "food", "Limit": 5000, "Period": "2026/10"},
	           {"Category": "furniture", "Limit": 200000, "Period": "2026/10"}]`
	if err := LoadBudgets(strings.NewReader(input)); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(budgets) != 2 {
		t.Errorf("expected 2 budgets, got %d", len(budgets))
	}
	if budgets["furniture"].Limit != 200000 {
		t.Errorf("unexpected furniture limit: %v", budgets["furniture"].Limit)
	}

	// Некорректный JSON: ожидается ошибка, бюджеты не меняются
	resetState()
	if err := LoadBudgets(strings.NewReader("not json")); err == nil {
		t.Error("expected parse error, got nil")
	}
	if len(budgets) != 0 {
		t.Errorf("expected no budgets after failed load, got %d", len(budgets))
	}
}
