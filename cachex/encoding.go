package cachex

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SetJSON encodes value as JSON and stores it in cache.
func SetJSON[T any](
	ctx context.Context,
	cache Cache,
	key string,
	value T,
	ttl time.Duration,
) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cachex: encode JSON: %w", err)
	}
	if err := cache.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("cachex: set JSON %q: %w", key, err)
	}
	return nil
}

// GetJSON loads and decodes a JSON value from cache.
func GetJSON[T any](ctx context.Context, cache Cache, key string) (T, error) {
	var value T
	data, err := cache.Get(ctx, key)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("cachex: decode JSON %q: %w", key, err)
	}
	return value, nil
}
