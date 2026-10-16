package service

import (
	"context"
	"warehouse/backend/internal/modules/item/domain"
)

type ItemStore interface {
	Create(context.Context, domain.Write) (domain.Item, error)
	List(context.Context, domain.ListParams) (domain.ListResult, error)
	Get(context.Context, int32) (domain.Item, error)
	Update(context.Context, int32, domain.Write) (domain.Item, error)
	Delete(context.Context, int32) (domain.Deleted, error)
}

type Service struct{ store ItemStore }

func New(store ItemStore) *Service { return &Service{store: store} }

func (s *Service) Create(ctx context.Context, w domain.Write) (domain.Item, error) {
	w, err := domain.NormalizeWrite(w)
	if err != nil {
		return domain.Item{}, err
	}
	return s.store.Create(ctx, w)
}
func (s *Service) List(ctx context.Context, p domain.ListParams) (domain.ListResult, error) {
	p, err := domain.NormalizeList(p)
	if err != nil {
		return domain.ListResult{}, err
	}
	return s.store.List(ctx, p)
}
func (s *Service) Get(ctx context.Context, id int32) (domain.Item, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.Item{}, err
	}
	return s.store.Get(ctx, id)
}
func (s *Service) Update(ctx context.Context, id int32, w domain.Write) (domain.Item, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.Item{}, err
	}
	w, err := domain.NormalizeWrite(w)
	if err != nil {
		return domain.Item{}, err
	}
	return s.store.Update(ctx, id, w)
}
func (s *Service) Delete(ctx context.Context, id int32) (domain.Deleted, error) {
	if err := domain.ValidateID(id); err != nil {
		return domain.Deleted{}, err
	}
	return s.store.Delete(ctx, id)
}
