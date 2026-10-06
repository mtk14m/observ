// Package wal implements a write-ahead log with group commit.
//
// Append returns only once the record is fsynced, so callers can safely
// acknowledge clients. Concurrent appends share fsync calls. The log is split
// into segments: Rotate seals the current segment, and Remove deletes sealed
// segments once their content is persisted elsewhere.
//
// Each record is framed as: length (uint32 LE) | CRC-32C of payload | payload.
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	headerSize = 8
	// MaxRecordSize protects replay from absurd lengths in corrupt data.
	MaxRecordSize = 256 << 20
	segmentExt    = ".wal"
)

var (
	// ErrClosed is returned by operations on a closed WAL.
	ErrClosed = errors.New("wal: closed")

	crcTable = crc32.MakeTable(crc32.Castagnoli)
)

// WAL is a segmented write-ahead log. It is safe for concurrent use.
type WAL struct {
	dir string

	// syncMu serialises fsyncs and segment changes. Lock order: syncMu, mu.
	syncMu sync.Mutex
	synced atomic.Uint64 // bytes known to be durable

	mu       sync.Mutex
	f        *os.File
	seg      uint64
	written  uint64 // bytes appended across all segments
	closed   bool
	previous []uint64 // segments that existed when the WAL was opened
}

// Open opens the WAL in dir, creating it if needed. Existing segments are
// kept for Replay; new records go to a fresh segment.
func Open(dir string) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("wal: create dir: %w", err)
	}
	segs, err := listSegments(dir)
	if err != nil {
		return nil, err
	}
	w := &WAL{dir: dir, previous: segs}
	next := uint64(1)
	if len(segs) > 0 {
		next = segs[len(segs)-1] + 1
	}
	if err := w.openSegment(next); err != nil {
		return nil, err
	}
	return w, nil
}

// Append durably writes payload. It returns after the record is fsynced.
func (w *WAL) Append(payload []byte) error {
	if len(payload) > MaxRecordSize {
		return fmt.Errorf("wal: record of %d bytes exceeds the %d bytes limit", len(payload), MaxRecordSize)
	}
	rec := make([]byte, headerSize+len(payload))
	binary.LittleEndian.PutUint32(rec[0:4], uint32(len(payload))) //nolint:gosec // bounded by MaxRecordSize
	binary.LittleEndian.PutUint32(rec[4:8], crc32.Checksum(payload, crcTable))
	copy(rec[headerSize:], payload)

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrClosed
	}
	if _, err := w.f.Write(rec); err != nil {
		w.mu.Unlock()
		return fmt.Errorf("wal: write: %w", err)
	}
	w.written += uint64(len(rec))
	target := w.written
	w.mu.Unlock()

	return w.syncUpTo(target)
}

// syncUpTo returns once at least target bytes are durable. A single fsync
// covers every append that completed before it started (group commit).
func (w *WAL) syncUpTo(target uint64) error {
	if w.synced.Load() >= target {
		return nil
	}
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	if w.synced.Load() >= target {
		return nil
	}
	w.mu.Lock()
	f, written, closed := w.f, w.written, w.closed
	w.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}
	w.synced.Store(written)
	return nil
}

// Rotate seals the current segment and starts a new one. It returns a seal
// value to pass to Remove once everything appended before the call has been
// persisted elsewhere.
func (w *WAL) Rotate() (seal uint64, err error) {
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, ErrClosed
	}
	if err := w.closeSegment(); err != nil {
		return 0, err
	}
	next := w.seg + 1
	if err := w.openSegment(next); err != nil {
		return 0, err
	}
	return next, nil
}

// Remove deletes every segment older than seal.
func (w *WAL) Remove(seal uint64) error {
	segs, err := listSegments(w.dir)
	if err != nil {
		return err
	}
	for _, id := range segs {
		if id >= seal {
			break
		}
		if err := os.Remove(w.segmentPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("wal: remove segment %d: %w", id, err)
		}
	}
	return nil
}

// Replay calls fn for every record of the segments that existed when the
// WAL was opened, in order. A torn or corrupt record ends the replay of its
// segment: it can only be the result of a crash during a write that was
// never acknowledged.
func (w *WAL) Replay(fn func(payload []byte) error) error {
	for _, id := range w.previous {
		if err := replaySegment(w.segmentPath(id), fn); err != nil {
			return err
		}
	}
	return nil
}

// Close syncs and closes the WAL.
func (w *WAL) Close() error {
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.closeSegment()
}

// closeSegment syncs and closes the current file. Callers hold both locks.
func (w *WAL) closeSegment() error {
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}
	w.synced.Store(w.written)
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("wal: close segment: %w", err)
	}
	return nil
}

func (w *WAL) openSegment(id uint64) error {
	f, err := os.OpenFile(w.segmentPath(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("wal: open segment: %w", err)
	}
	if err := syncDir(w.dir); err != nil {
		_ = f.Close()
		return err
	}
	w.f, w.seg = f, id
	return nil
}

func (w *WAL) segmentPath(id uint64) string {
	return filepath.Join(w.dir, fmt.Sprintf("%016d%s", id, segmentExt))
}

func replaySegment(path string, fn func([]byte) error) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("wal: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var header [headerSize]byte
	for {
		if _, err := io.ReadFull(f, header[:]); err != nil {
			return nil // clean end of segment, or torn header
		}
		n := binary.LittleEndian.Uint32(header[0:4])
		if n > MaxRecordSize {
			return nil
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(f, payload); err != nil {
			return nil // torn payload
		}
		if crc32.Checksum(payload, crcTable) != binary.LittleEndian.Uint32(header[4:8]) {
			return nil
		}
		if err := fn(payload); err != nil {
			return err
		}
	}
}

func listSegments(dir string) ([]uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("wal: list segments: %w", err)
	}
	var ids []uint64
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), segmentExt)
		if !ok || e.IsDir() {
			continue
		}
		if id, err := strconv.ParseUint(name, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("wal: open dir: %w", err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("wal: sync dir: %w", err)
	}
	return nil
}
