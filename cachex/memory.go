package cachex

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"
)

// Memory is a thread-safe in-memory Cache.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

// NewMemory creates an empty in-memory cache.
func NewMemory() *Memory {
	return &Memory{
		entries: make(map[string]entry),
		now:     time.Now,
	}
}

// Get returns a copy of the cached value.
func (c *Memory) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	item, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, ErrMiss
	}
	if !item.expiresAt.IsZero() && !c.now().Before(item.expiresAt) {
		c.mu.Lock()
		current, exists := c.entries[key]
		if exists && current.expiresAt.Equal(item.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil, ErrMiss
	}
	return bytes.Clone(item.value), nil
}

// Set stores a copy of value. Zero TTL does not expire; negative TTL deletes.
func (c *Memory) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" {
		return errors.New("cachex: key is empty")
	}
	if ttl < 0 {
		return c.Delete(ctx, key)
	}
	item := entry{value: bytes.Clone(value)}
	if ttl > 0 {
		item.expiresAt = c.now().Add(ttl)
	}
	c.mu.Lock()
	c.entries[key] = item
	c.mu.Unlock()
	return nil
}

// Delete removes a key. Deleting a missing key succeeds.
func (c *Memory) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" {
		return errors.New("cachex: key is empty")
	}
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
	return nil
}

var _ Cache = (*Memory)(nil)
