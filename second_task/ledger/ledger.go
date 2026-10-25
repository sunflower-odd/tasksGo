package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

var budgets = make(map[string]Budget)
var ErrBudgetExceeded = errors.New("budget exceeded")

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

	if err := tx.Validate(); err != nil {
		return err
	}

	budget, ok := budgets[tx.Category]

	if ok {
		if sumTransactions(tx.Category)+tx.Amount > budget.Limit {
			return ErrBudgetExceeded
		}
	}

	transactions = append(transactions, tx)
	return nil
}

// присваивает транзакции новый ID, добавляет её и возвращает созданную транзакцию
func CreateTransaction(tx Transaction) (Transaction, error) {
	tx.ID = len(transactions) + 1

	if err := AddTransaction(tx); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func ListTransactions() []Transaction {
	return transactions
}

func ListBudgets() []Budget {
	result := make([]Budget, 0, len(budgets))
	for _, b := range budgets {
		result = append(result, b)
	}
	return result
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
