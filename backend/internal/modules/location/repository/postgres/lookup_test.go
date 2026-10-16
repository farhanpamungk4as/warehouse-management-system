package postgres

import "testing"

func TestLookupRejectsNilTransaction(t *testing.T) {
	if lookup, err := NewLookup(nil); err == nil || lookup != nil {
		t.Fatal("lookup accepted nil transaction")
	}
}
