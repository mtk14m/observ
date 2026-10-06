// Package objstore defines the object storage abstraction used for all
// persisted telemetry. Implementations include the local filesystem and S3.
package objstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	// ErrNotFound is returned when an object does not exist.
	ErrNotFound = errors.New("objstore: object not found")
	// ErrInvalidKey is returned when a key does not satisfy ValidateKey.
	ErrInvalidKey = errors.New("objstore: invalid key")
)

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key     string
	Size    int64
	ModTime time.Time
}

// ObjectStore is a flat key/value store for immutable blobs.
//
// Put is atomic: readers see either the previous object or the complete new
// one, never a partial write. Delete of a missing key succeeds, as in S3.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// List returns the objects whose key starts with prefix, sorted by key.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

// ValidateKey reports whether key is a valid object key: a non-empty,
// slash-separated relative path whose segments are non-empty and do not
// start with a dot, without backslashes or NUL bytes.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if strings.ContainsAny(key, "\\\x00") {
		return fmt.Errorf("%w: %q contains a forbidden character", ErrInvalidKey, key)
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || strings.HasPrefix(seg, ".") {
			return fmt.Errorf("%w: %q has an empty or dot-prefixed segment", ErrInvalidKey, key)
		}
	}
	return nil
}
