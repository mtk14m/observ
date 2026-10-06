package ingest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/layout"
)

// Unflushed rows are made queryable through "hot" snapshots: Parquet files
// in HotDir holding the rows still in memory (buffered, or being flushed).
// A snapshot is rebuilt when the buffer changed and is invalidated by every
// flush, so a row is never visible both in a snapshot and in the store,
// except during the few milliseconds between a flush's write and its
// completion.

type snapshot struct {
	path  string
	gen   uint64
	epoch uint64
	built time.Time
}

// HotFiles returns the local Parquet files holding the unflushed rows of a
// signal directory (layout.Logs, layout.Spans or layout.MetricPoints).
func (p *Pipeline) HotFiles(dir string) ([]string, error) {
	p.hotMu.Lock()
	defer p.hotMu.Unlock()

	p.bufMu.Lock()
	gen, epoch := p.gen[dir], p.epoch
	snap := p.hot[dir]
	fresh := snap != nil && snap.epoch == epoch &&
		(snap.gen == gen || (p.opts.HotRefresh > 0 && time.Since(snap.built) < p.opts.HotRefresh))
	if fresh {
		p.bufMu.Unlock()
		return snap.paths(), nil
	}
	var (
		data []byte
		n    int
		err  error
	)
	switch dir {
	case layout.Logs:
		n = len(p.logs) + len(p.flushingLogs)
		data, err = encode(n, p.flushingLogs, p.logs)
	case layout.Spans:
		n = len(p.spans) + len(p.flushingSpans)
		data, err = encode(n, p.flushingSpans, p.spans)
	case layout.MetricPoints:
		n = len(p.points) + len(p.flushingPoints)
		data, err = encode(n, p.flushingPoints, p.points)
	default:
		p.bufMu.Unlock()
		return nil, fmt.Errorf("ingest: unknown signal directory %q", dir)
	}
	p.bufMu.Unlock()
	if err != nil {
		return nil, err
	}

	next := &snapshot{gen: gen, epoch: epoch, built: time.Now()}
	if n > 0 {
		p.hotSeq++
		next.path = filepath.Join(p.opts.HotDir, fmt.Sprintf("%s-%d.parquet", dir, p.hotSeq))
		if err := writeFileAtomic(next.path, data); err != nil {
			return nil, err
		}
	}
	// Keep the previous snapshot: a query may still be reading it.
	if snap != nil && p.hotPrev[dir] != "" {
		_ = os.Remove(p.hotPrev[dir])
	}
	if snap != nil {
		p.hotPrev[dir] = snap.path
	}
	p.hot[dir] = next
	return next.paths(), nil
}

func (s *snapshot) paths() []string {
	if s.path == "" {
		return nil
	}
	return []string{s.path}
}

// encode writes the rows of every slice as one Parquet file, or nothing.
func encode[T any](n int, slices ...[]T) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	rows := make([]T, 0, n)
	for _, s := range slices {
		rows = append(rows, s...)
	}
	var buf bytes.Buffer
	if err := parquet.Write(&buf, rows); err != nil {
		return nil, fmt.Errorf("ingest: snapshot: %w", err)
	}
	return buf.Bytes(), nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("ingest: snapshot: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ingest: snapshot: %w", err)
	}
	return nil
}
