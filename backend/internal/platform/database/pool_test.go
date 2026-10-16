package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOpenIsBoundedAndSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	pool, err := Open(ctx, "postgres://test:secret@127.0.0.1:1/test?sslmode=disable", 2, 50*time.Millisecond)
	if pool != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected safe unavailable error, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("initialization exceeded its bound")
	}
}
