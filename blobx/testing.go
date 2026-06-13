package blobx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

// MemoryStore is a race-safe in-memory Store suitable for application tests.
// Stored and returned bytes are copied.
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

type memoryObject struct {
	data []byte
	info Object
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string]memoryObject)}
}

// Put stores an object, replacing any existing value.
func (s *MemoryStore) Put(
	ctx context.Context,
	key string,
	body io.Reader,
	opts PutOptions,
) (Object, error) {
	key, err := NormalizeKey(key)
	if err != nil {
		return Object{}, err
	}
	var buffer bytes.Buffer
	hashing := &hashWriter{}
	size, err := copyContext(ctx, io.MultiWriter(&buffer, hashing), body)
	if err != nil {
		return Object{}, err
	}
	info := Object{
		Key:          key,
		Size:         size,
		ContentType:  opts.ContentType,
		ETag:         hashing.Sum(),
		LastModified: time.Now().UTC(),
	}
	s.mu.Lock()
	s.objects[key] = memoryObject{data: bytes.Clone(buffer.Bytes()), info: info}
	s.mu.Unlock()
	return info, nil
}

// Get returns a copied object body.
func (s *MemoryStore) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, Object{}, err
	}
	key, err := NormalizeKey(key)
	if err != nil {
		return nil, Object{}, err
	}
	s.mu.RLock()
	object, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, Object{}, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(object.data))), object.info, nil
}

// Stat returns stored metadata.
func (s *MemoryStore) Stat(ctx context.Context, key string) (Object, error) {
	body, info, err := s.Get(ctx, key)
	if body != nil {
		_ = body.Close()
	}
	return info, err
}

// Delete removes an object. Deleting a missing object succeeds.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

type hashWriter struct {
	data []byte
}

func (w *hashWriter) Write(data []byte) (int, error) {
	w.data = append(w.data, data...)
	return len(data), nil
}

func (w *hashWriter) Sum() string {
	if w == nil {
		return ""
	}
	sum := sha256.Sum256(w.data)
	return hex.EncodeToString(sum[:])
}

var _ Store = (*MemoryStore)(nil)
