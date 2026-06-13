package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/r6m/gox/cachex"
	"github.com/r6m/gox/cachex/internal/cachetest"
	goredis "github.com/redis/go-redis/v9"
)

func TestCache(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache, err := New(client, Options{KeyPrefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	cachetest.Run(t, cache)
	ctx := context.Background()
	if _, err := cache.Get(ctx, "missing"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("unexpected miss: %v", err)
	}
	input := []byte("value")
	if err := cache.Set(ctx, "key", input, time.Second); err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	got, err := cache.Get(ctx, "key")
	if err != nil || string(got) != "value" {
		t.Fatalf("unexpected value: %q %v", got, err)
	}
	server.FastForward(time.Second)
	if _, err := cache.Get(ctx, "key"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("expired value: %v", err)
	}
}

func TestProviderFailureIsNotMiss(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	cache, err := New(client, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = cache.Get(ctx, "key")
	if err == nil || errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("provider failure became miss: %v", err)
	}
}
