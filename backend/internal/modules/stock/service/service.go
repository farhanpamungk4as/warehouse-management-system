package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"warehouse/backend/internal/modules/stock/domain"
)

type ItemGuard interface {
	EnsureActiveForReceive(context.Context, int32) error
}
type LocationLookup interface {
	EnsureExistsForReceive(context.Context, int32) error
}
type StockWriter interface {
	Increment(context.Context, int32, int32, int32) (domain.Stock, error)
}
type ReceiveSession struct {
	Item     ItemGuard
	Location LocationLookup
	Stock    StockWriter
}
type ReceiveRunner interface {
	Run(context.Context, func(ReceiveSession) (domain.Stock, error)) (domain.Stock, error)
}
type StockReader interface {
	Lookup(context.Context, int32) (domain.Lookup, error)
}

type Service struct {
	reader    StockReader
	runner    ReceiveRunner
	logger    *slog.Logger
	requestID func(context.Context) string
}

func New(reader StockReader, runner ReceiveRunner, logger *slog.Logger, requestID func(context.Context) string) *Service {
	return &Service{reader: reader, runner: runner, logger: logger, requestID: requestID}
}
func (s *Service) Receive(ctx context.Context, input domain.ReceiveInput) (domain.Stock, error) {
	if err := domain.ValidateReceive(input); err != nil {
		return domain.Stock{}, err
	}
	stock, err := s.runner.Run(ctx, func(session ReceiveSession) (domain.Stock, error) {
		if err := session.Item.EnsureActiveForReceive(ctx, int32(input.ItemID)); err != nil {
			return domain.Stock{}, &domain.ReferenceError{Field: "item_id", Cause: err}
		}
		if err := session.Location.EnsureExistsForReceive(ctx, int32(input.LocationID)); err != nil {
			return domain.Stock{}, &domain.ReferenceError{Field: "location_id", Cause: err}
		}
		return session.Stock.Increment(ctx, int32(input.ItemID), int32(input.LocationID), int32(input.Qty))
	})
	if err != nil {
		var unknown *domain.CommitOutcomeUnknown
		if errors.As(err, &unknown) {
			s.logger.ErrorContext(ctx, "stock.receive.commit_unknown", "event", "stock.receive.commit_unknown", "request_id", s.requestID(ctx), "item_id", input.ItemID, "location_id", input.LocationID, "received_qty", input.Qty, "timestamp", time.Now().UTC())
		}
		return domain.Stock{}, err
	}
	// The runner returns a result only after receiving a successful COMMIT reply.
	s.logger.InfoContext(ctx, "stock.receive.committed", "event", "stock.receive.committed", "request_id", s.requestID(ctx), "item_id", stock.ItemID, "location_id", stock.LocationID, "stock_id", stock.ID, "received_qty", input.Qty, "qty_before", int64(stock.Qty)-input.Qty, "qty_after", stock.Qty, "timestamp", time.Now().UTC())
	return stock, nil
}
func (s *Service) Lookup(ctx context.Context, itemID int64) (domain.Lookup, error) {
	if err := domain.ValidateID("item_id", itemID); err != nil {
		return domain.Lookup{}, err
	}
	return s.reader.Lookup(ctx, int32(itemID))
}
