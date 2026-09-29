package main

import (
	"errors"
	"fmt"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

var transactions []Transaction

func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("Amount must not equal 0!")
	}

	transactions = append(transactions, tx)
	return nil
}

func ListTransactions() []Transaction {
	return transactions
}

func main() {
	fmt.Println("Ledger service started")
	transactions = []Transaction{}

	AddTransaction(Transaction{1, 30.0, "asset", "покупка", "2026-10-01"})
	AddTransaction(Transaction{2, 40.0, "profit", "profit", "2026-10-11"})
	AddTransaction(Transaction{3, 0, "profit", "profit", "2026-10-11"})
	AddTransaction(Transaction{4, 50.0, "profit", "profit", "2026-10-12"})
	fmt.Println(ListTransactions())
}
