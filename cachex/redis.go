package cachex

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisOptions configures a Redis-backed cache.
type RedisOptions struct {
	KeyPrefix string
}

// RedisCache stores cache values in Redis.
type RedisCache struct {
	client goredis.UniversalClient
	prefix string
}

// NewRedis creates a Redis-backed cache.
func NewRedis(client goredis.UniversalClient, opts RedisOptions) (*RedisCache, error) {
	if client == nil {
		return nil, errors.New("cachex/redis: client is required")
	}
	return &RedisCache{client: client, prefix: opts.KeyPrefix}, nil
}

// Get returns a copied cached value.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, errors.New("cachex/redis: key is empty")
	}
	data, err := c.client.Get(ctx, c.key(key)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrMiss
	}
	if err != nil {
		return nil, fmt.Errorf("cachex/redis: get %q: %w", key, err)
	}
	return append([]byte(nil), data...), nil
}

// Set stores value. Zero TTL persists; negative TTL deletes.
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if key == "" {
		return errors.New("cachex/redis: key is empty")
	}
	if ttl < 0 {
		return c.Delete(ctx, key)
	}
	if err := c.client.Set(ctx, c.key(key), append([]byte(nil), value...), ttl).Err(); err != nil {
		return fmt.Errorf("cachex/redis: set %q: %w", key, err)
	}
	return nil
}

// Delete removes a key. Deleting a missing key succeeds.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("cachex/redis: key is empty")
	}
	if err := c.client.Del(ctx, c.key(key)).Err(); err != nil {
		return fmt.Errorf("cachex/redis: delete %q: %w", key, err)
	}
	return nil
}

func (c *RedisCache) key(key string) string {
	return c.prefix + key
}

var _ Cache = (*RedisCache)(nil)
