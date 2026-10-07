// Package cache keeps local copies of remote objects so that DuckDB, which
// reads local files, can query an object store such as S3.
//
// Objects written by obsrv are immutable (every file has a unique key), so a
// cached copy never goes stale. Copies are evicted least-recently-used first
// once the cache exceeds MaxBytes, except copies served within the grace
// period, which a query may be about to open.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/mtk14n/obsrv/internal/objstore"
)

const (
	defaultMaxBytes = 2 << 30 // 2 GiB
	defaultGrace    = time.Minute
)

// Options configures a Cache.
type Options struct {
	Store objstore.ObjectStore
	// Dir holds the local copies. It is cleared on start.
	Dir string
	// MaxBytes is the target size of the cache. Defaults to 2 GiB.
	MaxBytes int64
	// Grace protects recently served copies from eviction. Defaults to one
	// minute; a negative value disables it.
	Grace time.Duration
}

type entry struct {
	path     string
	size     int64
	lastUsed time.Time
}

// Cache wraps an object store with local copies of the objects it serves.
type Cache struct {
	opts    Options
	group   singleflight.Group
	mu      sync.Mutex
	entries map[string]*entry
	total   int64
}

// New creates the cache directory, removing copies left by a previous run.
func New(opts Options) (*Cache, error) {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = defaultMaxBytes
	}
	if opts.Grace == 0 {
		opts.Grace = defaultGrace
	}
	if err := os.RemoveAll(opts.Dir); err != nil {
		return nil, fmt.Errorf("cache: clear: %w", err)
	}
	if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("cache: create: %w", err)
	}
	return &Cache{opts: opts, entries: map[string]*entry{}}, nil
}

// List lists objects in the underlying store.
func (c *Cache) List(ctx context.Context, prefix string) ([]objstore.ObjectInfo, error) {
	return c.opts.Store.List(ctx, prefix)
}

// Local returns the path of a local copy of the object, downloading it on
// first use. Concurrent calls for the same key share one download.
func (c *Cache) Local(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		e.lastUsed = time.Now()
		c.mu.Unlock()
		return e.path, nil
	}
	c.mu.Unlock()

	v, err, _ := c.group.Do(key, func() (any, error) {
		return c.download(ctx, key)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *Cache) download(ctx context.Context, key string) (string, error) {
	rc, err := c.opts.Store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()

	sum := sha256.Sum256([]byte(key))
	path := filepath.Join(c.opts.Dir, hex.EncodeToString(sum[:16])+filepath.Ext(key))
	tmp, err := os.CreateTemp(c.opts.Dir, "dl-*")
	if err != nil {
		return "", fmt.Errorf("cache: %w", err)
	}
	size, err := io.Copy(tmp, rc)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("cache: download %q: %w", key, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &entry{path: path, size: size, lastUsed: time.Now()}
	c.total += size
	c.evictLocked(key)
	return path, nil
}

// evictLocked removes least recently used copies until the cache fits,
// keeping the copy just added and those still in their grace period.
func (c *Cache) evictLocked(keep string) {
	if c.total <= c.opts.MaxBytes {
		return
	}
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return c.entries[keys[i]].lastUsed.Before(c.entries[keys[j]].lastUsed) })
	now := time.Now()
	for _, k := range keys {
		if c.total <= c.opts.MaxBytes {
			return
		}
		e := c.entries[k]
		if k == keep || (c.opts.Grace > 0 && now.Sub(e.lastUsed) < c.opts.Grace) {
			continue
		}
		_ = os.Remove(e.path)
		c.total -= e.size
		delete(c.entries, k)
	}
}
