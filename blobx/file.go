package blobx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FileOptions configures a filesystem-backed Store.
type FileOptions struct {
	Root string
}

// FileStore stores objects under a filesystem root.
type FileStore struct {
	root string
}

// NewFileStore creates a filesystem-backed store.
func NewFileStore(opts FileOptions) (*FileStore, error) {
	if opts.Root == "" {
		return nil, errors.New("blobx: filesystem root is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("blobx: resolve filesystem root: %w", err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("blobx: create filesystem root: %w", err)
	}
	return &FileStore{root: root}, nil
}

// Put streams body to key, replacing any existing object.
func (s *FileStore) Put(
	ctx context.Context,
	key string,
	body io.Reader,
	opts PutOptions,
) (Object, error) {
	key, err := NormalizeKey(key)
	if err != nil {
		return Object{}, err
	}
	if body == nil {
		return Object{}, errors.New("blobx: body is nil")
	}
	target := s.objectPath(key)
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return Object{}, fmt.Errorf("blobx: create object parent: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".blobx-*")
	if err != nil {
		return Object{}, fmt.Errorf("blobx: create temporary object: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	hash := sha256.New()
	size, copyErr := copyContext(ctx, io.MultiWriter(temp, hash), body)
	closeErr := temp.Close()
	if copyErr != nil {
		return Object{}, fmt.Errorf("blobx: write object: %w", copyErr)
	}
	if closeErr != nil {
		return Object{}, fmt.Errorf("blobx: close object: %w", closeErr)
	}
	if err := os.Rename(tempName, target); err != nil {
		return Object{}, fmt.Errorf("blobx: replace object: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return Object{}, fmt.Errorf("blobx: stat written object: %w", err)
	}
	object := Object{
		Key:          key,
		Size:         size,
		ContentType:  opts.ContentType,
		ETag:         hex.EncodeToString(hash.Sum(nil)),
		LastModified: info.ModTime(),
	}
	if err := writeMetadata(s.metadataPath(key), object); err != nil {
		return Object{}, err
	}
	return object, nil
}

// Get opens an object for streaming. The caller must close the returned body.
func (s *FileStore) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, Object{}, err
	}
	object, err := s.Stat(ctx, key)
	if err != nil {
		return nil, Object{}, err
	}
	file, err := os.Open(s.objectPath(object.Key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Object{}, ErrNotFound
	}
	if err != nil {
		return nil, Object{}, fmt.Errorf("blobx: open object: %w", err)
	}
	return file, object, nil
}

// Stat returns object metadata.
func (s *FileStore) Stat(ctx context.Context, key string) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	key, err := NormalizeKey(key)
	if err != nil {
		return Object{}, err
	}
	info, err := os.Stat(s.objectPath(key))
	if errors.Is(err, fs.ErrNotExist) {
		return Object{}, ErrNotFound
	}
	if err != nil {
		return Object{}, fmt.Errorf("blobx: stat object: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Object{}, ErrNotFound
	}
	object := Object{Key: key, Size: info.Size(), LastModified: info.ModTime()}
	metadata, err := readMetadata(s.metadataPath(key))
	if err == nil {
		object.ContentType = metadata.ContentType
		object.ETag = metadata.ETag
	}
	return object, nil
}

// Delete removes an object. Deleting a missing object succeeds.
func (s *FileStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(s.objectPath(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("blobx: delete object: %w", err)
	}
	if err := os.Remove(s.metadataPath(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("blobx: delete object metadata: %w", err)
	}
	return nil
}

func (s *FileStore) objectPath(key string) string {
	return filepath.Join(s.root, ".blobx", "objects", filepath.FromSlash(key))
}

func (s *FileStore) metadataPath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.root, ".blobx", "metadata", hex.EncodeToString(sum[:]))
}

func writeMetadata(path string, object Object) error {
	data, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("blobx: encode object metadata: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("blobx: create metadata directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("blobx: write object metadata: %w", err)
	}
	return nil
}

func readMetadata(path string) (Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Object{}, err
	}
	var object Object
	if err := json.Unmarshal(data, &object); err != nil {
		return Object{}, err
	}
	return object, nil
}

func copyContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := src.Read(buffer)
		if read > 0 {
			count, writeErr := dst.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

var _ Store = (*FileStore)(nil)
