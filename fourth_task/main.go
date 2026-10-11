package fourthtask

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type Validatable interface {
    Validate() error
}

type Transaction struct {
	Amount float64
	Category    string
	Date   string
}

func (Transaction) Validate {
	if Amount == 0 {
	return errors.New("Amount must not equal 0!")
	}

}


func main() {
	fmt.Println("Ledger service started")
}