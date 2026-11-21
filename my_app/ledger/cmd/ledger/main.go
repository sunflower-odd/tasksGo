package main

import (
	"bufio"
	"context"
	"fmt"
	"ledger"
	"ledger/internal/app"

	"os"
)

func main() {
	fmt.Println("Ledger service started")

	ctx := context.Background()
	svc, closeFn, err := app.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not init ledger: %v\n", err)
		os.Exit(1)
	}
	defer closeFn()
	fmt.Println("Connected to database")

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

	for _, budget := range []ledger.Budget{
		{Category: "food", Limit: 5000, Period: "2026/10"},
		{Category: "electronics", Limit: 100000, Period: "2026/10"},
		{Category: "furniture", Limit: 200000, Period: "2026/10"},
	} {
		if err := svc.SetBudget(ctx, budget); err != nil {
			fmt.Printf("Could not set budget for %q: %v\n", budget.Category, err)
			return
		}
	}

	f, err := os.Open("budgets.json")
	if err != nil {
		fmt.Println("file can not be open:", err)
		return
	}
	defer f.Close()

	err = ledger.LoadBudgets(ctx, svc, bufio.NewReader(f))

	if err != nil {
		fmt.Println("file can not be read:", err)
		return
	}

	transactions, err := svc.ListTransactions(ctx)
	if err != nil {
		fmt.Println("Could not list transactions:", err)
		return
	}

	for _, tx := range []ledger.Transaction{
		{Amount: 3000, Category: "food", Description: "покупка", Date: "2026-10-01"},
		{Amount: 60000, Category: "electronics", Description: "profit", Date: "2026-10-11"},
		{Amount: 0, Category: "profit", Description: "profit", Date: "2026-10-11"},
		{Amount: 50, Category: "profit", Description: "profit", Date: "2026-10-12"},
	} {
		exists := false
		for _, existing := range transactions {
			if existing.Amount == tx.Amount && existing.Category == tx.Category &&
				existing.Description == tx.Description && existing.Date == tx.Date {
				exists = true
				break
			}
		}
		if exists {
			fmt.Printf("Transaction already exists: %s, %s, %.2f\n", tx.Date, tx.Category, tx.Amount)
			continue
		}
		created, err := svc.AddTransaction(ctx, tx)
		if err != nil {
			fmt.Println("Transaction has been rejected:", err)
			continue
		}
		transactions = append(transactions, created)
	}

	fmt.Println(transactions)

	for i := 0; i < 2; i++ {
		report, err := svc.GetReportSummary(ctx, "2026-10-01", "2026-10-31")
		if err != nil {
			fmt.Println("Could not get report:", err)
			continue
		}
		fmt.Printf("Report %d: %+v\n", i+1, report)
	}
}
