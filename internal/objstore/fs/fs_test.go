package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/objstore/objstoretest"
)

func TestConformance(t *testing.T) {
	objstoretest.Run(t, func(t *testing.T) objstore.ObjectStore {
		s, err := fs.New(t.TempDir())
		if err != nil {
			t.Fatalf("fs.New: %v", err)
		}
		return s
	})
}

func TestNewCreatesRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "store")
	if _, err := fs.New(root); err != nil {
		t.Fatalf("fs.New: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("root directory not created: %v", err)
	}
}

func TestObjectsAreStoredAsPlainFiles(t *testing.T) {
	// The on-disk layout is part of the public contract: an object with key K
	// must be readable at <root>/K by any tool, without obsrv.
	root := t.TempDir()
	s, err := fs.New(root)
	if err != nil {
		t.Fatalf("fs.New: %v", err)
	}
	if err := s.Put(t.Context(), "v1/logs/x.parquet", stringsReader("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "v1", "logs", "x.parquet"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "data" {
		t.Errorf("file content = %q, want %q", b, "data")
	}
}

func TestDeleteRemovesEmptyParentDirectories(t *testing.T) {
	root := t.TempDir()
	s, err := fs.New(root)
	if err != nil {
		t.Fatalf("fs.New: %v", err)
	}
	if err := s.Put(t.Context(), "a/b/c", stringsReader("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Delete(t.Context(), "a/b/c"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Errorf("empty parent directory %q still exists (err=%v)", "a", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("root must never be removed: %v", err)
	}
}

func TestLocalPathPointsToTheObjectFile(t *testing.T) {
	root := t.TempDir()
	s, err := fs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(t.Context(), "a/b.parquet", stringsReader("x")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.LocalPath("a/b.parquet"))
	if err != nil || string(b) != "x" {
		t.Errorf("LocalPath content = %q, %v; want %q", b, err, "x")
	}
}
