package database

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Classify preserves unexpected causes for centralized safe presentation.
func Classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if strings.HasPrefix(pg.Code, "08") || pg.Code == "57P01" || pg.Code == "57P02" || pg.Code == "57P03" || pg.Code == "53300" {
			return ErrUnavailable
		}
		return err
	}
	var network net.Error
	var connect *pgconn.ConnectError
	if errors.As(err, &network) || errors.As(err, &connect) || pgconn.SafeToRetry(err) {
		return ErrUnavailable
	}
	return err
}
