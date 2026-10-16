package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"warehouse/backend/internal/modules/location/domain"
)

const CacheKey = "warehouse:v1:locations"

type Reader interface {
	List(context.Context) ([]domain.Location, error)
}
type Store interface {
	Reader
	Get(context.Context, int32) (domain.Location, error)
	Create(context.Context, domain.Write) (domain.Location, error)
	Update(context.Context, int32, domain.Write) (domain.Location, error)
	Delete(context.Context, int32) (domain.Location, error)
}
type Cache interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
}
type Service struct {
	repo        Store
	cache       Cache
	ttl         time.Duration
	mu          sync.RWMutex
	bypassUntil time.Time
}

func New(repo Store, cache Cache, ttl time.Duration) *Service {
	return &Service{repo: repo, cache: cache, ttl: ttl}
}
func (s *Service) List(ctx context.Context) ([]domain.Location, error) {
	// A reader cannot refill an old snapshot after a local mutation invalidates it.
	s.mu.RLock()
	defer s.mu.RUnlock()
	useCache := s.cache != nil && !time.Now().Before(s.bypassUntil)
	if useCache {
		bounded, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		data, err := s.cache.Get(bounded, CacheKey)
		cancel()
		var result []domain.Location
		if err == nil && json.Unmarshal(data, &result) == nil && result != nil {
			valid := true
			for _, v := range result {
				if v.ID < 1 || v.Code == "" {
					valid = false
					break
				}
			}
			if valid {
				return result, nil
			}
		}
	}
	result, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.Location{}
	}
	if useCache && s.ttl > 0 {
		data, err := json.Marshal(result)
		if err == nil {
			bounded, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			_ = s.cache.Set(bounded, CacheKey, data, s.ttl)
			cancel()
		}
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id int32) (domain.Location, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) invalidate(ctx context.Context) {
	if s.cache == nil {
		return
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 100*time.Millisecond)
	defer cancel()
	if err := s.cache.Delete(bounded, CacheKey); err != nil {
		// A failed cache purge must neither report a committed write as failed,
		// nor let this process serve the old key while its TTL is still valid.
		s.bypassUntil = time.Now().Add(s.ttl)
	}
}
func (s *Service) Create(ctx context.Context, w domain.Write) (domain.Location, error) {
	w, err := domain.NormalizeWrite(w)
	if err != nil {
		return domain.Location{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.repo.Create(ctx, w)
	if err == nil {
		s.invalidate(ctx)
	}
	return v, err
}
func (s *Service) Update(ctx context.Context, id int32, w domain.Write) (domain.Location, error) {
	w, err := domain.NormalizeWrite(w)
	if err != nil {
		return domain.Location{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.repo.Update(ctx, id, w)
	if err == nil {
		s.invalidate(ctx)
	}
	return v, err
}
func (s *Service) Delete(ctx context.Context, id int32) (domain.Location, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.repo.Delete(ctx, id)
	if err == nil {
		s.invalidate(ctx)
	}
	return v, err
}
