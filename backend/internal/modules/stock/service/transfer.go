package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"warehouse/backend/internal/modules/stock/domain"
)

type TransferLocations interface {
	LockForTransfer(context.Context, int32, int32) error
}
type TransferWriter interface {
	Move(context.Context, domain.TransferInput) (domain.Transfer, error)
}
type TransferSession struct {
	Item      ItemGuard
	Locations TransferLocations
	Stock     TransferWriter
}
type TransferRunner interface {
	RunTransfer(context.Context, func(TransferSession) (domain.Transfer, error)) (domain.Transfer, error)
}
type TransferService struct {
	runner    TransferRunner
	logger    *slog.Logger
	requestID func(context.Context) string
}

func NewTransfer(runner TransferRunner, logger *slog.Logger, id func(context.Context) string) *TransferService {
	return &TransferService{runner, logger, id}
}
func (s *TransferService) Transfer(ctx context.Context, in domain.TransferInput) (domain.Transfer, error) {
	if err := domain.ValidateTransfer(in); err != nil {
		return domain.Transfer{}, err
	}
	result, err := s.runner.RunTransfer(ctx, func(tx TransferSession) (domain.Transfer, error) {
		if err := tx.Item.EnsureActiveForReceive(ctx, int32(in.ItemID)); err != nil {
			return domain.Transfer{}, &domain.ReferenceError{Field: "item_id", Cause: err}
		}
		if err := tx.Locations.LockForTransfer(ctx, int32(in.FromLocationID), int32(in.ToLocationID)); err != nil {
			return domain.Transfer{}, err
		}
		return tx.Stock.Move(ctx, in)
	})
	if err != nil {
		var unknown *domain.CommitOutcomeUnknown
		if errors.As(err, &unknown) {
			unknown.Operation = "transfer"
			s.logger.ErrorContext(ctx, "stock.transfer.commit_unknown", "event", "stock.transfer.commit_unknown", "request_id", s.requestID(ctx), "item_id", in.ItemID, "from_location_id", in.FromLocationID, "to_location_id", in.ToLocationID, "qty", in.Qty, "timestamp", time.Now().UTC())
		}
		return domain.Transfer{}, err
	}
	s.logger.InfoContext(ctx, "stock.transfer.committed", "event", "stock.transfer.committed", "request_id", s.requestID(ctx), "item_id", result.ItemID, "from_location_id", result.FromLocationID, "to_location_id", result.ToLocationID, "qty", result.Qty, "source_qty_after", result.Source.Qty, "destination_qty_after", result.Destination.Qty, "timestamp", time.Now().UTC())
	return result, nil
}
