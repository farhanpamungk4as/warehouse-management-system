package domain

import "errors"

var (
	ErrNotFound     = errors.New("item not found")
	ErrDuplicateSKU = errors.New("SKU already exists")
)

type FieldError struct {
	Field  string
	Reason string
}

type ValidationError struct{ Fields []FieldError }

func (*ValidationError) Error() string { return "Validation failed" }
