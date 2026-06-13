// Package cachex provides a small byte-oriented cache abstraction.
package cachex

import (
	"context"
	"errors"
	"time"
)

// ErrMiss indicates that a cache key does not exist or has expired.
var ErrMiss = errors.New("cachex: cache miss")

// Cache stores byte slices by key.
//
// Implementations must not expose mutable ownership of stored values. Set
// copies input bytes when necessary, and Get returns bytes owned by the caller.
// A zero TTL means no expiration. A negative TTL deletes the key.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
