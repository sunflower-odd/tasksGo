package main

import (
	"bufio"
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

type Validatable interface {
	Validate() error
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("Amount must be greater than 0!")
	}

	if tx.Category == "" {
		return errors.New("Category should not be empty!")
	}

	return nil

}

func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New("Limit must be greater than 0!")
	}

	if b.Category == "" {
		return errors.New("Category should not be empty!")
	}

	return nil

}

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
	// if tx.Amount == 0 {
	// 	return errors.New("Amount must not equal 0!")
	// }

	if err := tx.Validate(); err != nil {
		return err
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

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}

	budgets[b.Category] = b
	return nil
}

func LoadBudgets(r io.Reader) error {
	var loadedBudgets []Budget
	err := json.NewDecoder(r).Decode(&loadedBudgets)

	if err != nil {
		return fmt.Errorf("budget file can not be parsed: %w", err)
	}

	for _, budget := range loadedBudgets {
		if err := SetBudget(budget); err != nil {
			return fmt.Errorf("invalid budget for category %q: %w", budget.Category, err)
		}
	}
	return nil
}

func CheckValid(v Validatable) error {
	err := v.Validate()
	if err != nil {
		return fmt.Errorf("Object could not be validated: %w", err)
	}
	return nil
}

func main() {
	fmt.Println("Ledger service started")

	transactions = []Transaction{}
	budgets = make(map[string]Budget)

	tx := Transaction{
		ID:       10,
		Amount:   100,
		Category: "food",
		Date:     "2026-10-10",
	}

	budget := Budget{
		Category: "food",
		Limit:    5000,
		Period:   "2026/10",
	}

	if err := CheckValid(tx); err != nil {
		fmt.Println("Transaction validation failed:", err)
	} else {
		fmt.Println("Transaction is valid")
	}

	if err := CheckValid(budget); err != nil {
		fmt.Println("Budget validation failed:", err)
	} else {
		fmt.Println("Budget is valid")
	}

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

	err = LoadBudgets(bufio.NewReader(f))

	if err != nil {
		fmt.Println("file can not be read:", err)
		return
	}

	err = AddTransaction(Transaction{1, 3000.0, "food", "покупка", "2026-10-01"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = AddTransaction(Transaction{2, 60000.0, "electronics", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = AddTransaction(Transaction{3, 60000.0, "electronics", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = AddTransaction(Transaction{4, 0, "profit", "profit", "2026-10-11"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}
	err = AddTransaction(Transaction{5, 50.0, "profit", "profit", "2026-10-12"})
	if err != nil {
		fmt.Println("Transaction has been rejected:", err)
	}

	fmt.Println(ListTransactions())
}
