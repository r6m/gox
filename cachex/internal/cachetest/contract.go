// Package cachetest provides shared cachex adapter contract tests.
package cachetest

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r6m/gox/cachex"
)

// Run exercises the provider-neutral cachex.Cache contract.
func Run(t testing.TB, cache cachex.Cache) {
	t.Helper()
	ctx := context.Background()
	if _, err := cache.Get(ctx, "missing"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("missing get: %v", err)
	}
	input := []byte("value")
	if err := cache.Set(ctx, "key", input, 0); err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	first, err := cache.Get(ctx, "key")
	if err != nil || !bytes.Equal(first, []byte("value")) {
		t.Fatalf("unexpected value: %q %v", first, err)
	}
	first[0] = 'Y'
	second, err := cache.Get(ctx, "key")
	if err != nil || !bytes.Equal(second, []byte("value")) {
		t.Fatalf("returned bytes were not copied: %q %v", second, err)
	}
	if err := cache.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, "key"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("deleted get: %v", err)
	}
	if err := cache.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, "key", []byte("deleted"), -time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, "key"); !errors.Is(err, cachex.ErrMiss) {
		t.Fatalf("negative TTL did not delete: %v", err)
	}
}
