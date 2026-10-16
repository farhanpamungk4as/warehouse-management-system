package service

import (
	"context"
	"strings"
	"testing"
	"warehouse/backend/internal/modules/item/domain"
)

type storeSpy struct {
	ItemStore
	calls int
	write domain.Write
}

func (s *storeSpy) Create(_ context.Context, w domain.Write) (domain.Item, error) {
	s.calls++
	s.write = w
	return domain.Item{}, nil
}
func (s *storeSpy) Update(_ context.Context, _ int32, w domain.Write) (domain.Item, error) {
	s.calls++
	s.write = w
	return domain.Item{}, nil
}
func TestIndependentDomainValidation(t *testing.T) {
	store := &storeSpy{}
	s := New(store)
	ctx := context.Background()
	good := domain.Write{SKU: " abc ", Name: " 箱 ", Category: " Tools ", Unit: " pcs "}
	if _, err := s.Create(ctx, good); err != nil || store.write.SKU != "abc" || store.write.Name != "箱" {
		t.Fatalf("%+v %v", store.write, err)
	}
	store.calls = 0
	for _, v := range []string{"", " ", "\u2003\u00a0", "bad\nvalue", strings.Repeat("界", 65)} {
		w := good
		w.SKU = v
		if _, err := s.Create(ctx, w); err == nil {
			t.Fatal("create accepted invalid SKU")
		}
		if _, err := s.Update(ctx, 1, w); err == nil {
			t.Fatal("update accepted invalid SKU")
		}
	}
	for _, id := range []int32{0, -1} {
		if _, err := s.Get(ctx, id); err == nil {
			t.Fatal("get ID")
		}
		if _, err := s.Delete(ctx, id); err == nil {
			t.Fatal("delete ID")
		}
		if _, err := s.Update(ctx, id, good); err == nil {
			t.Fatal("update ID")
		}
	}
	for _, p := range []domain.ListParams{{Page: 0, Limit: 10}, {Page: 1, Limit: 101}, {Page: domain.MaxID, Limit: 100}, {Page: 9223372036854775807, Limit: 100}, {Page: 1, Limit: 10, Query: "\t"}} {
		if _, err := s.List(ctx, p); err == nil {
			t.Fatal("invalid list accepted")
		}
	}
	if store.calls != 0 {
		t.Fatal("invalid input reached store")
	}
	good.SKU = strings.Repeat("界", 64)
	good.Name = strings.Repeat("界", 200)
	good.Category = strings.Repeat("界", 64)
	good.Unit = strings.Repeat("界", 32)
	if _, err := s.Update(ctx, 1, good); err != nil {
		t.Fatal("Unicode character bounds rejected", err)
	}
}
