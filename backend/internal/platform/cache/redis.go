// Package cache contains only transport configuration; location caching policy
// remains in the consuming location service. Stock is never cached.
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct{ client *redis.Client }

func Open(raw string) (*Redis, error) {
	options, err := redis.ParseURL(raw)
	if err != nil {
		return nil, err
	}
	options.DialTimeout = 100 * time.Millisecond
	options.ReadTimeout = 100 * time.Millisecond
	options.WriteTimeout = 100 * time.Millisecond
	options.PoolTimeout = 100 * time.Millisecond
	options.MaxRetries = -1
	options.ContextTimeoutEnabled = true
	return &Redis{redis.NewClient(options)}, nil
}
func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	return r.client.Get(ctx, key).Bytes()
}
func (r *Redis) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, data, ttl).Err()
}
func (r *Redis) Close() error                                 { return r.client.Close() }
func (r *Redis) Delete(ctx context.Context, key string) error { return r.client.Del(ctx, key).Err() }
