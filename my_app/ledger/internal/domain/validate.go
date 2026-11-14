package domain

import "fmt"

type Validatable interface {
	Validate() error
}

func CheckValid(v Validatable) error {
	err := v.Validate()
	if err != nil {
		return fmt.Errorf("object could not be validated: %w", err)
	}
	return nil
}
