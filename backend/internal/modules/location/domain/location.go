package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrNotFound = errors.New("location not found")
var ErrDuplicateCode = errors.New("location code already exists")
var ErrInUse = errors.New("location has stock records")

const MaxID = 2147483647

type Write struct {
	Code string `json:"code"`
	Zone string `json:"zone"`
	Type string `json:"type"`
}
type FieldError struct{ Field, Reason string }
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "location validation failed" }

func NormalizeWrite(w Write) (Write, error) {
	var fields []FieldError
	for _, f := range []struct {
		name  string
		value *string
		max   int
	}{
		{"code", &w.Code, 64}, {"zone", &w.Zone, 64}, {"type", &w.Type, 32},
	} {
		*f.value = strings.Trim(*f.value, " ")
		reason := ""
		switch {
		case !utf8.ValidString(*f.value):
			reason = "Must be valid UTF-8"
		case strings.TrimSpace(*f.value) == "":
			reason = "Must not be blank"
		case utf8.RuneCountInString(*f.value) > f.max:
			reason = "Exceeds maximum character length"
		default:
			for _, r := range *f.value {
				if r < 0x20 || r == 0x7f {
					reason = "Must not contain control characters"
					break
				}
			}
		}
		if reason != "" {
			fields = append(fields, FieldError{f.name, reason})
		}
	}
	if len(fields) > 0 {
		return Write{}, &ValidationError{fields}
	}
	return w, nil
}

type Location struct {
	ID   int32  `json:"id"`
	Code string `json:"code"`
	Zone string `json:"zone"`
	Type string `json:"type"`
}
