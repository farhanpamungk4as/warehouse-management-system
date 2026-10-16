package postgres

import "testing"

func TestReceiveGuardRejectsNilTransaction(t *testing.T) {
	if guard, err := NewReceiveGuard(nil); err == nil || guard != nil {
		t.Fatal("guard accepted nil transaction")
	}
}
