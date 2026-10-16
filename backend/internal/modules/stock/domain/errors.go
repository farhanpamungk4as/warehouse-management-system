package domain

import "errors"

var ErrCapacityExceeded = errors.New("stock quantity limit exceeded")

type FieldError struct{ Field, Reason string }
type ValidationError struct{ Fields []FieldError }

func (*ValidationError) Error() string { return "Validation failed" }

// ReferenceError annotates a field without changing the owning domain's identity.
type ReferenceError struct {
	Field string
	Cause error
}

func (*ReferenceError) Error() string   { return "stock reference unavailable" }
func (e *ReferenceError) Unwrap() error { return e.Cause }

// CommitOutcomeUnknown must take precedence over its cause in HTTP error mapping.
type CommitOutcomeUnknown struct {
	Cause     error
	Operation string
}

func (*CommitOutcomeUnknown) Error() string   { return "stock commit outcome unknown" }
func (e *CommitOutcomeUnknown) Unwrap() error { return e.Cause }
