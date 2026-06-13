package cachex_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r6m/gox/cachex"
	"github.com/r6m/gox/cachex/internal/cachetest"
)

func TestMemoryContract(t *testing.T) {
	cachetest.Run(t, cachex.NewMemory())
}

func TestMemoryExpiration(t *testing.T) {
	cache := cachex.NewMemory()
	if err := cache.Set(context.Background(), "key", []byte("value"), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := cache.Get(context.Background(), "key"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("expired get: %v", err)
	}
}

func TestMemoryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cache := cachex.NewMemory()
	if err := cache.Set(ctx, "key", []byte("value"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("set cancellation: %v", err)
	}
	if _, err := cache.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("get cancellation: %v", err)
	}
}

func TestJSON(t *testing.T) {
	type value struct {
		Name string `json:"name"`
	}
	cache := cachex.NewMemory()
	if err := cachex.SetJSON(context.Background(), cache, "value", value{Name: "test"}, 0); err != nil {
		t.Fatal(err)
	}
	got, err := cachex.GetJSON[value](context.Background(), cache, "value")
	if err != nil || got.Name != "test" {
		t.Fatalf("unexpected JSON: %#v %v", got, err)
	}
}
