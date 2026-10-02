package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

var transactions []Transaction

type Budget struct {
	Category string
	Limit    float64
	Period   string
}

var budgets map[string]Budget

func sumTransactions(category string) float64 {
	var total float64

	for _, tx := range transactions {
		if tx.Category == category {
			total += tx.Amount
		}
	}

	return total
}

func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("Amount must not equal 0!")
	}

	budget, ok := budgets[tx.Category]

	if ok {
		if sumTransactions(tx.Category)+tx.Amount > budget.Limit {
			return errors.New("budget exceeded")
		}
	}

	transactions = append(transactions, tx)
	return nil
}

func ListTransactions() []Transaction {
	return transactions
}

func SetBudget(b Budget) {
	budgets[b.Category] = b
}

func LoadBudgets(r io.Reader) error {
	var loadedBudgets []Budget
	err := json.NewDecoder(r).Decode(&loadedBudgets)

	if err != nil {
		return errors.New("budget file can not be parsed")
	}

	for _, budget := range loadedBudgets {
		SetBudget(budget)
	}
	return nil
}

func main() {
	fmt.Println("Ledger service started")

	transactions = []Transaction{}
	budgets = make(map[string]Budget)

	SetBudget(Budget{
		Category: "food",
		Limit:    5000.0,
		Period:   "2026/10",
	})

	SetBudget(Budget{
		Category: "electronics",
		Limit:    100000.0,
		Period:   "2026/10",
	})

	SetBudget(Budget{
		Category: "furniture",
		Limit:    200000.0,
		Period:   "2026/10",
	})

	f, err := os.Open("budgets.json")
	if err != nil {
		fmt.Println("file can not be open:", err)
		return
	}
	defer f.Close()

	err = LoadBudgets(f)
	if err != nil {
		fmt.Println("file can not be read:", err)
		return
	}

	AddTransaction(Transaction{1, 3000.0, "food", "покупка", "2026-10-01"})
	AddTransaction(Transaction{2, 60000.0, "electronics", "profit", "2026-10-11"})
	AddTransaction(Transaction{2, 60000.0, "electronics", "profit", "2026-10-11"})
	AddTransaction(Transaction{3, 0, "profit", "profit", "2026-10-11"})
	AddTransaction(Transaction{4, 50.0, "profit", "profit", "2026-10-12"})

	fmt.Println(ListTransactions())
}
