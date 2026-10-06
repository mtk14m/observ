package wal_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/mtk14n/obsrv/internal/wal"
)

func open(t *testing.T, dir string) *wal.WAL {
	t.Helper()
	w, err := wal.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return w
}

func replay(t *testing.T, w *wal.WAL) []string {
	t.Helper()
	var got []string
	if err := w.Replay(func(p []byte) error {
		got = append(got, string(p))
		return nil
	}); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	return got
}

func appendAll(t *testing.T, w *wal.WAL, payloads ...string) {
	t.Helper()
	for _, p := range payloads {
		if err := w.Append([]byte(p)); err != nil {
			t.Fatalf("Append(%q): %v", p, err)
		}
	}
}

func TestReplayAfterReopenReturnsRecordsInOrder(t *testing.T) {
	dir := t.TempDir()
	w := open(t, dir)
	appendAll(t, w, "a", "b", "c")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	w = open(t, dir)
	defer func() { _ = w.Close() }()
	if got, want := replay(t, w), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("Replay = %v, want %v", got, want)
	}
}

func TestReplayOfFreshWALIsEmpty(t *testing.T) {
	w := open(t, t.TempDir())
	defer func() { _ = w.Close() }()
	if got := replay(t, w); len(got) != 0 {
		t.Errorf("Replay = %v, want empty", got)
	}
}

func TestConcurrentAppendsAreAllDurable(t *testing.T) {
	dir := t.TempDir()
	w := open(t, dir)
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			if err := w.Append(fmt.Appendf(nil, "%03d", i)); err != nil {
				t.Errorf("Append: %v", err)
			}
		})
	}
	wg.Wait()
	_ = w.Close()

	w = open(t, dir)
	defer func() { _ = w.Close() }()
	got := replay(t, w)
	slices.Sort(got)
	if len(got) != 100 || got[0] != "000" || got[99] != "099" {
		t.Errorf("replayed %d records (%v…), want 100", len(got), got[:min(3, len(got))])
	}
}

func TestRemoveDropsSealedSegmentsOnly(t *testing.T) {
	dir := t.TempDir()
	w := open(t, dir)
	appendAll(t, w, "flushed-1", "flushed-2")
	seal, err := w.Rotate()
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	appendAll(t, w, "pending")
	if err := w.Remove(seal); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_ = w.Close()

	w = open(t, dir)
	defer func() { _ = w.Close() }()
	if got, want := replay(t, w), []string{"pending"}; !slices.Equal(got, want) {
		t.Errorf("Replay = %v, want %v", got, want)
	}
}

func TestTornTailIsIgnored(t *testing.T) {
	dir := t.TempDir()
	w := open(t, dir)
	appendAll(t, w, "complete", "torn-record")
	_ = w.Close()

	// Simulate a crash in the middle of the last write.
	segs, _ := filepath.Glob(filepath.Join(dir, "*.wal"))
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %v", segs)
	}
	info, _ := os.Stat(segs[0])
	if err := os.Truncate(segs[0], info.Size()-3); err != nil {
		t.Fatal(err)
	}

	w = open(t, dir)
	defer func() { _ = w.Close() }()
	if got, want := replay(t, w), []string{"complete"}; !slices.Equal(got, want) {
		t.Errorf("Replay = %v, want %v", got, want)
	}
	// The WAL keeps working after recovery.
	appendAll(t, w, "after-crash")
}

func TestCorruptedRecordStopsSegmentReplay(t *testing.T) {
	dir := t.TempDir()
	w := open(t, dir)
	appendAll(t, w, "good", "bad!")
	_ = w.Close()

	segs, _ := filepath.Glob(filepath.Join(dir, "*.wal"))
	b, _ := os.ReadFile(segs[0])
	b[len(b)-1] ^= 0xff // flip a payload byte: the checksum no longer matches
	if err := os.WriteFile(segs[0], b, 0o600); err != nil {
		t.Fatal(err)
	}

	w = open(t, dir)
	defer func() { _ = w.Close() }()
	if got, want := replay(t, w), []string{"good"}; !slices.Equal(got, want) {
		t.Errorf("Replay = %v, want %v", got, want)
	}
}

func TestAppendAfterCloseFails(t *testing.T) {
	w := open(t, t.TempDir())
	_ = w.Close()
	if err := w.Append([]byte("x")); err == nil {
		t.Error("Append after Close returned nil error")
	}
}
