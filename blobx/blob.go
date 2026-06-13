// Package blobx provides streaming object storage abstractions.
package blobx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// ErrNotFound indicates that an object does not exist.
var ErrNotFound = errors.New("blobx: object not found")

// Object contains provider-neutral object metadata.
type Object struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

// PutOptions controls object writes. Put overwrites an existing key.
type PutOptions struct {
	ContentType string
}

// Store provides streaming object storage.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, opts PutOptions) (Object, error)
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	Stat(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
}

// PresignOptions controls optional presigned read URLs.
type PresignOptions struct {
	Expires time.Duration
}

// Presigner is an optional capability implemented by stores that can create
// temporary read URLs.
type Presigner interface {
	PresignGet(ctx context.Context, key string, opts PresignOptions) (string, error)
}

// NormalizeKey validates an object key and returns its canonical form.
//
// Keys use forward slashes, must be relative, and may not contain empty,
// current-directory, or parent-directory segments.
func NormalizeKey(key string) (string, error) {
	if key == "" {
		return "", errors.New("blobx: key is empty")
	}
	if strings.ContainsRune(key, '\x00') || strings.Contains(key, "\\") {
		return "", errors.New("blobx: key contains invalid characters")
	}
	if strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
		return "", errors.New("blobx: key must be a relative object name")
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", errors.New("blobx: key contains an invalid path segment")
		}
	}
	normalized := path.Clean(key)
	if normalized == "." || normalized != key {
		return "", fmt.Errorf("blobx: key %q is not canonical", key)
	}
	return normalized, nil
}
