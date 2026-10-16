package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

const MaxID = 2147483647

type Item struct {
	ID        int32     `json:"id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Unit      string    `json:"unit"`
	CreatedAt time.Time `json:"created_at"`
}

type Write struct{ SKU, Name, Category, Unit string }

type Deleted struct {
	ID        int32     `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type ListParams struct {
	Page     int64
	Limit    int64
	Category string
	Query    string
}

type ListResult struct {
	Items      []Item
	Total      int64
	Categories []string
}

// NormalizeText follows the same text policy for writes and optional filters.
func NormalizeText(field, value string, max int, required bool) (string, *FieldError) {
	value = strings.Trim(value, " ")
	var reason string
	switch {
	case !utf8.ValidString(value):
		reason = "Must be valid UTF-8"
	case (required || value != "") && strings.TrimSpace(value) == "":
		reason = "Must not be blank"
	case utf8.RuneCountInString(value) > max:
		reason = "Exceeds maximum character length"
	default:
		for _, r := range value {
			if r < 0x20 || r == 0x7f {
				reason = "Must not contain control characters"
				break
			}
		}
	}
	if reason != "" {
		return value, &FieldError{Field: field, Reason: reason}
	}
	return value, nil
}

func NormalizeWrite(w Write) (Write, error) {
	var fields []FieldError
	for _, f := range []struct {
		name  string
		value *string
		max   int
	}{
		{"sku", &w.SKU, 64}, {"name", &w.Name, 200}, {"category", &w.Category, 64}, {"unit", &w.Unit, 32},
	} {
		value, err := NormalizeText(f.name, *f.value, f.max, true)
		*f.value = value
		if err != nil {
			fields = append(fields, *err)
		}
	}
	if len(fields) > 0 {
		return Write{}, &ValidationError{Fields: fields}
	}
	return w, nil
}

func ValidateID(id int32) error {
	if id < 1 {
		return &ValidationError{Fields: []FieldError{{"id", "Must be a positive 32-bit integer"}}}
	}
	return nil
}

func NormalizeList(p ListParams) (ListParams, error) {
	var fields []FieldError
	if p.Page < 1 {
		fields = append(fields, FieldError{"page", "Must be a positive integer"})
	}
	if p.Limit < 1 || p.Limit > 100 {
		fields = append(fields, FieldError{"limit", "Must be between 1 and 100"})
	}
	// Divide before multiplying so even an int64-sized page cannot wrap.
	if len(fields) == 0 && p.Page-1 > MaxID/p.Limit {
		fields = append(fields, FieldError{"page", "Pagination offset exceeds maximum"})
	}
	for _, f := range []struct {
		name  string
		value *string
		max   int
	}{{"category", &p.Category, 64}, {"q", &p.Query, 200}} {
		value, err := NormalizeText(f.name, *f.value, f.max, false)
		*f.value = value
		if err != nil {
			fields = append(fields, *err)
		}
	}
	if len(fields) > 0 {
		return ListParams{}, &ValidationError{Fields: fields}
	}
	return p, nil
}
