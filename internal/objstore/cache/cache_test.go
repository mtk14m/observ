package cache_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/cache"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
)

// countingStore counts downloads.
type countingStore struct {
	objstore.ObjectStore
	gets atomic.Int64
}

func (s *countingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.gets.Add(1)
	time.Sleep(5 * time.Millisecond) // make concurrent callers overlap
	return s.ObjectStore.Get(ctx, key)
}

func setup(t *testing.T, opts cache.Options, objects map[string]string) (*cache.Cache, *countingStore) {
	t.Helper()
	backing, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range objects {
		if err := backing.Put(t.Context(), k, strings.NewReader(v)); err != nil {
			t.Fatal(err)
		}
	}
	store := &countingStore{ObjectStore: backing}
	opts.Store = store
	opts.Dir = t.TempDir()
	c, err := cache.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return c, store
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDownloadsOnceThenServesFromDisk(t *testing.T) {
	c, store := setup(t, cache.Options{MaxBytes: 1 << 20}, map[string]string{"v1/logs/a.parquet": "AAAA"})
	for range 3 {
		path, err := c.Local(t.Context(), "v1/logs/a.parquet")
		if err != nil {
			t.Fatal(err)
		}
		if got := read(t, path); got != "AAAA" {
			t.Errorf("content = %q", got)
		}
	}
	if n := store.gets.Load(); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}
}

func TestConcurrentCallersShareOneDownload(t *testing.T) {
	c, store := setup(t, cache.Options{MaxBytes: 1 << 20}, map[string]string{"k": "data"})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := c.Local(t.Context(), "k"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := store.gets.Load(); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}
}

func TestEvictsLeastRecentlyUsedBeyondMaxBytes(t *testing.T) {
	c, _ := setup(t, cache.Options{MaxBytes: 10, Grace: -1}, map[string]string{
		"a": "1234", "b": "1234", "c": "1234",
	})
	pa, _ := c.Local(t.Context(), "a")
	pb, _ := c.Local(t.Context(), "b")
	_, _ = c.Local(t.Context(), "a") // a is now more recent than b
	_, _ = c.Local(t.Context(), "c") // 12 bytes > 10: evict b

	if _, err := os.Stat(pb); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("b should have been evicted (err=%v)", err)
	}
	if _, err := os.Stat(pa); err != nil {
		t.Errorf("a should still be cached: %v", err)
	}
}

func TestRecentlyServedFilesAreNotEvicted(t *testing.T) {
	c, _ := setup(t, cache.Options{MaxBytes: 4, Grace: time.Hour}, map[string]string{"a": "1234", "b": "1234"})
	pa, _ := c.Local(t.Context(), "a")
	_, _ = c.Local(t.Context(), "b")
	// A query may still be about to open a: it must survive, even over the limit.
	if _, err := os.Stat(pa); err != nil {
		t.Errorf("a was evicted while still in its grace period: %v", err)
	}
}

func TestMissingObject(t *testing.T) {
	c, _ := setup(t, cache.Options{}, nil)
	if _, err := c.Local(t.Context(), "nope"); !errors.Is(err, objstore.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListIsDelegated(t *testing.T) {
	c, _ := setup(t, cache.Options{}, map[string]string{"v1/x": "1"})
	infos, err := c.List(t.Context(), "v1/")
	if err != nil || len(infos) != 1 {
		t.Errorf("List = %v, %v", infos, err)
	}
}

func TestStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "stale.parquet")
	_ = os.WriteFile(stale, []byte("x"), 0o600)
	backing, _ := fs.New(t.TempDir())
	if _, err := cache.New(cache.Options{Store: backing, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Error("files from a previous run must be cleared")
	}
}
