package main

import (
	"bufio"
	"fmt"
	"ledger"
	"os"
)

func main() {
	fmt.Println("Ledger service started")

	tx := ledger.Transaction{
		ID:       10,
		Amount:   100,
		Category: "food",
		Date:     "2026-10-10",
	}

	budget := ledger.Budget{
		Category: "food",
		Limit:    5000,
		Period:   "2026/10",
	}

	if err := ledger.CheckValid(tx); err != nil {
		fmt.Println("Transaction validation failed:", err)
	} else {
		fmt.Println("Transaction is valid")
	}

	if err := ledger.CheckValid(budget); err != nil {
		fmt.Println("Budget validation failed:", err)
	} else {
		fmt.Println("Budget is valid")
	}

	ledger.SetBudget(ledger.Budget{
		Category: "food",
		Limit:    5000.0,
		Period:   "2026/10",
	})

	ledger.SetBudget(ledger.Budget{
		Category: "electronics",
		Limit:    100000.0,
		Period:   "2026/10",
	})

	ledger.SetBudget(ledger.Budget{
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

	err = ledger.LoadBudgets(bufio.NewReader(f))

	if err != nil {
		fmt.Println("file can not be read:", err)
		return
	}

	err = ledger.AddTransaction(ledger.Transaction{1, 3000.0, "food", "покупка", "2026-10-01"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = ledger.AddTransaction(ledger.Transaction{2, 60000.0, "electronics", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = ledger.AddTransaction(ledger.Transaction{3, 60000.0, "electronics", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = ledger.AddTransaction(ledger.Transaction{4, 0, "profit", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = ledger.AddTransaction(ledger.Transaction{5, 50.0, "profit", "profit", "2026-10-12"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}

	fmt.Println(ledger.ListTransactions())
}
