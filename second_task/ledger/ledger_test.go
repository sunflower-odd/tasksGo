package main

import "testing"

func TestAddTransaction(t *testing.T) {

	transactions = []Transaction{}
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
