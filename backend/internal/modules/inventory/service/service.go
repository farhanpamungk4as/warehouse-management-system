package service

import (
	"context"

	"warehouse/backend/internal/modules/inventory/domain"
	item "warehouse/backend/internal/modules/item/domain"
)

type Reader interface {
	Summary(context.Context) (domain.Summary, error)
	Overview(context.Context, domain.Params) (domain.Overview, error)
}
type Service struct{ repo Reader }

func New(repo Reader) *Service                                         { return &Service{repo} }
func (s *Service) Summary(ctx context.Context) (domain.Summary, error) { return s.repo.Summary(ctx) }
func (s *Service) Overview(ctx context.Context, p domain.Params) (domain.Overview, error) {
	normalized, err := item.NormalizeList(p.ListParams)
	if err != nil {
		return domain.Overview{}, err
	}
	p.ListParams = normalized
	if p.LocationID < 0 {
		return domain.Overview{}, &item.ValidationError{Fields: []item.FieldError{{Field: "location_id", Reason: "Must be positive"}}}
	}
	return s.repo.Overview(ctx, p)
}
