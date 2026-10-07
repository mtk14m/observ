// Package fs implements objstore.ObjectStore on the local filesystem.
//
// An object with key K is stored as the plain file <root>/K so that the data
// stays readable by any tool, without obsrv.
package fs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mtk14n/obsrv/internal/objstore"
)

// tmpDir holds in-flight writes. Its name is not a valid key, so it can
// never collide with an object and is skipped by List.
const tmpDir = ".tmp"

// Store is a filesystem-backed object store.
type Store struct {
	root string
}

var _ objstore.ObjectStore = (*Store)(nil)

// New returns a store rooted at root, creating the directory if needed.
func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("objstore/fs: resolve root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, tmpDir), 0o750); err != nil {
		return nil, fmt.Errorf("objstore/fs: create root: %w", err)
	}
	return &Store{root: abs}, nil
}

// Put writes the object atomically: content goes to a temporary file that is
// synced and then renamed over the destination.
func (s *Store) Put(ctx context.Context, key string, r io.Reader) (err error) {
	if err := objstore.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Join(s.root, tmpDir), "put-*")
	if err != nil {
		return fmt.Errorf("objstore/fs: put %q: %w", key, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("objstore/fs: put %q: write: %w", key, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("objstore/fs: put %q: sync: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("objstore/fs: put %q: close: %w", key, err)
	}

	dst := s.path(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("objstore/fs: put %q: mkdir: %w", key, err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("objstore/fs: put %q: rename: %w", key, err)
	}
	return nil
}

// Get opens the object for reading. The caller must close it.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := objstore.ValidateKey(key); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", objstore.ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("objstore/fs: get %q: %w", key, err)
	}
	return f, nil
}

// Delete removes the object and any parent directories left empty.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := objstore.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p := s.path(key)
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("objstore/fs: delete %q: %w", key, err)
	}
	s.removeEmptyParents(p)
	return nil
}

// removeEmptyParents removes the directories between p and the root, stopping
// at the first one that is not empty. Failures are harmless and ignored.
func (s *Store) removeEmptyParents(p string) {
	for dir := filepath.Dir(p); dir != s.root; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

// List walks the store and returns the objects whose key starts with prefix.
func (s *Store) List(ctx context.Context, prefix string) ([]objstore.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []objstore.ObjectInfo
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == tmpDir && filepath.Dir(p) == s.root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, objstore.ObjectInfo{Key: key, Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("objstore/fs: list %q: %w", prefix, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *Store) path(key string) string {
	return filepath.Join(s.root, filepath.FromSlash(key))
}

// LocalPath returns the path of the file holding the object with key. It
// lets local readers such as DuckDB open objects directly.
func (s *Store) LocalPath(key string) string {
	return s.path(key)
}

// Local implements the query engine's file source: objects are already
// local files.
func (s *Store) Local(_ context.Context, key string) (string, error) {
	if err := objstore.ValidateKey(key); err != nil {
		return "", err
	}
	return s.path(key), nil
}
