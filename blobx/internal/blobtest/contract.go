// Package blobtest provides shared blobx adapter contract tests.
package blobtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/r6m/gox/blobx"
)

// Run exercises the provider-neutral blobx.Store contract.
func Run(t testing.TB, store blobx.Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.Stat(ctx, "missing.txt"); !errors.Is(err, blobx.ErrNotFound) {
		t.Fatalf("missing stat: %v", err)
	}
	first, err := store.Put(ctx, "folder/object.txt", bytes.NewBufferString("first"), blobx.PutOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Key != "folder/object.txt" || first.Size != 5 {
		t.Fatalf("unexpected put metadata: %#v", first)
	}
	body, info, err := store.Get(ctx, first.Key)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil || closeErr != nil || string(data) != "first" || info.Size != 5 {
		t.Fatalf("unexpected get: %q %#v %v %v", data, info, readErr, closeErr)
	}
	if _, err := store.Put(ctx, first.Key, bytes.NewBufferString("replacement"), blobx.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	body, _, err = store.Get(ctx, first.Key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = io.ReadAll(body)
	_ = body.Close()
	if string(data) != "replacement" {
		t.Fatalf("overwrite failed: %q", data)
	}
	if err := store.Delete(ctx, first.Key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(ctx, first.Key); !errors.Is(err, blobx.ErrNotFound) {
		t.Fatalf("deleted get: %v", err)
	}
	if err := store.Delete(ctx, first.Key); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}
