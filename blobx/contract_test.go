package blobx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/r6m/gox/blobx"
	"github.com/r6m/gox/blobx/internal/blobtest"
)

func TestMemoryStoreContract(t *testing.T) {
	blobtest.Run(t, blobx.NewMemoryStore())
}

func TestFileStoreContract(t *testing.T) {
	store, err := blobx.NewFileStore(blobx.FileOptions{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	blobtest.Run(t, store)
}

func TestNormalizeKey(t *testing.T) {
	valid := []string{"a", "folder/file.txt", "a-b_c.1"}
	for _, key := range valid {
		if got, err := blobx.NormalizeKey(key); err != nil || got != key {
			t.Fatalf("valid key %q: %q %v", key, got, err)
		}
	}
	invalid := []string{"", "/absolute", "trailing/", "../escape", "a/../b", "a//b", "a\\b", "./a"}
	for _, key := range invalid {
		if _, err := blobx.NormalizeKey(key); err == nil {
			t.Fatalf("invalid key accepted: %q", key)
		}
	}
}

func TestStoreCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := blobx.NewMemoryStore()
	if _, err := store.Put(ctx, "object", strings.NewReader("data"), blobx.PutOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
}
