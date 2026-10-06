// Package objstoretest provides a conformance suite that every
// objstore.ObjectStore implementation must pass.
package objstoretest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mtk14n/obsrv/internal/objstore"
)

// Factory returns a new, empty store for one subtest.
type Factory func(t *testing.T) objstore.ObjectStore

// Run executes the conformance suite against stores built by newStore.
func Run(t *testing.T, newStore Factory) {
	t.Helper()

	t.Run("PutThenGetReturnsContent", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "a/b.txt", "hello")
		if got := get(t, s, "a/b.txt"); got != "hello" {
			t.Errorf("Get = %q, want %q", got, "hello")
		}
	})

	t.Run("PutOverwritesExistingObject", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "k", "first")
		put(t, s, "k", "second")
		if got := get(t, s, "k"); got != "second" {
			t.Errorf("Get = %q, want %q", got, "second")
		}
	})

	t.Run("GetMissingReturnsErrNotFound", func(t *testing.T) {
		s := newStore(t)
		_, err := s.Get(context.Background(), "missing")
		if !errors.Is(err, objstore.ErrNotFound) {
			t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("DeleteRemovesObject", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "dir/k", "v")
		if err := s.Delete(context.Background(), "dir/k"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := s.Get(context.Background(), "dir/k"); !errors.Is(err, objstore.ErrNotFound) {
			t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
		}
	})

	t.Run("DeleteMissingIsNotAnError", func(t *testing.T) {
		s := newStore(t)
		if err := s.Delete(context.Background(), "missing"); err != nil {
			t.Errorf("Delete(missing) = %v, want nil", err)
		}
	})

	t.Run("ListReturnsMatchingKeysSorted", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "logs/b", "22")
		put(t, s, "logs/a", "1")
		put(t, s, "logs/sub/c", "333")
		put(t, s, "spans/x", "")

		got, err := s.List(context.Background(), "logs/")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		want := []objstore.ObjectInfo{
			{Key: "logs/a", Size: 1},
			{Key: "logs/b", Size: 2},
			{Key: "logs/sub/c", Size: 3},
		}
		assertInfos(t, got, want)
	})

	t.Run("ListWithEmptyPrefixReturnsEverything", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "b", "")
		put(t, s, "a/c", "")
		got, err := s.List(context.Background(), "")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		assertInfos(t, got, []objstore.ObjectInfo{{Key: "a/c"}, {Key: "b"}})
	})

	t.Run("ListOnEmptyStoreReturnsNothing", func(t *testing.T) {
		s := newStore(t)
		got, err := s.List(context.Background(), "nothing/")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("List = %v, want empty", got)
		}
	})

	t.Run("FailedPutKeepsPreviousObject", func(t *testing.T) {
		s := newStore(t)
		put(t, s, "k", "original")

		boom := errors.New("boom")
		r := io.MultiReader(strings.NewReader("partial"), errReader{boom})
		if err := s.Put(context.Background(), "k", r); !errors.Is(err, boom) {
			t.Fatalf("Put with failing reader error = %v, want %v", err, boom)
		}
		if got := get(t, s, "k"); got != "original" {
			t.Errorf("Get after failed Put = %q, want %q", got, "original")
		}
		infos, err := s.List(context.Background(), "")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		assertInfos(t, infos, []objstore.ObjectInfo{{Key: "k", Size: int64(len("original"))}})
	})

	t.Run("InvalidKeysAreRejected", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		const bad = "../escape"
		if err := s.Put(ctx, bad, strings.NewReader("x")); !errors.Is(err, objstore.ErrInvalidKey) {
			t.Errorf("Put error = %v, want ErrInvalidKey", err)
		}
		if _, err := s.Get(ctx, bad); !errors.Is(err, objstore.ErrInvalidKey) {
			t.Errorf("Get error = %v, want ErrInvalidKey", err)
		}
		if err := s.Delete(ctx, bad); !errors.Is(err, objstore.ErrInvalidKey) {
			t.Errorf("Delete error = %v, want ErrInvalidKey", err)
		}
	})

	t.Run("CanceledContextIsHonored", func(t *testing.T) {
		s := newStore(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := s.Put(ctx, "k", strings.NewReader("x")); !errors.Is(err, context.Canceled) {
			t.Errorf("Put error = %v, want context.Canceled", err)
		}
		if _, err := s.List(ctx, ""); !errors.Is(err, context.Canceled) {
			t.Errorf("List error = %v, want context.Canceled", err)
		}
	})
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func put(t *testing.T, s objstore.ObjectStore, key, content string) {
	t.Helper()
	if err := s.Put(context.Background(), key, strings.NewReader(content)); err != nil {
		t.Fatalf("Put(%q): %v", key, err)
	}
}

func get(t *testing.T, s objstore.ObjectStore, key string) string {
	t.Helper()
	rc, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get(%q): %v", key, err)
	}
	defer func() { _ = rc.Close() }()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	return buf.String()
}

// assertInfos compares keys and sizes; ModTime is implementation-defined.
func assertInfos(t *testing.T, got, want []objstore.ObjectInfo) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("List returned %d objects %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i].Key != want[i].Key || got[i].Size != want[i].Size {
			t.Errorf("List[%d] = {%q, %d}, want {%q, %d}", i, got[i].Key, got[i].Size, want[i].Key, want[i].Size)
		}
	}
}
