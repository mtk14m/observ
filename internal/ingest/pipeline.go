// Package ingest turns received telemetry into stored telemetry.
//
// Every batch is written to the WAL before it is acknowledged, then kept in
// memory. Flush seals the WAL, writes the buffered rows as Parquet files
// (one per signal and hour, sorted for compression) and only then drops the
// sealed WAL segments. Delivery is at-least-once: a crash during a flush can
// duplicate rows, never lose them.
package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/model"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/otlp"
	"github.com/mtk14n/obsrv/internal/wal"
	"github.com/mtk14n/obsrv/pkg/schema"
)

const (
	defaultFlushInterval   = 10 * time.Second
	defaultMaxBufferedRows = 500_000
)

// WAL record kinds.
const (
	kindLogs byte = iota + 1
	kindTraces
	kindMetrics
)

// Options configures a Pipeline.
type Options struct {
	WALDir string
	Store  objstore.ObjectStore
	// FlushInterval defaults to 10s.
	FlushInterval time.Duration
	// MaxBufferedRows is the per-signal limit above which ingestion applies
	// backpressure. Defaults to 500,000.
	MaxBufferedRows int
	Now             func() time.Time
	Logger          *slog.Logger
}

// Stats counts the items received per signal.
type Stats struct {
	Spans      int64
	DataPoints int64
	LogRecords int64
}

// Pipeline is an otlp.Sink that persists telemetry. Safe for concurrent use.
type Pipeline struct {
	opts Options
	wal  *wal.WAL

	// mu makes "append to WAL + append to buffer" atomic with respect to
	// "rotate WAL + swap buffers": consumers hold it shared, Flush exclusive.
	mu      sync.RWMutex
	bufMu   sync.Mutex
	logs    []schema.Log
	spans   []schema.Span
	points  []schema.MetricPoint
	flushMu sync.Mutex
	flushCh chan struct{}

	nSpans, nPoints, nLogs atomic.Int64
}

// New opens the WAL and replays any data that was not flushed before the
// previous shutdown.
func New(opts Options) (*Pipeline, error) {
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = defaultFlushInterval
	}
	if opts.MaxBufferedRows <= 0 {
		opts.MaxBufferedRows = defaultMaxBufferedRows
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	w, err := wal.Open(opts.WALDir)
	if err != nil {
		return nil, err
	}
	p := &Pipeline{opts: opts, wal: w, flushCh: make(chan struct{}, 1)}
	if err := w.Replay(p.replay); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("ingest: replay: %w", err)
	}
	return p, nil
}

// ConsumeLogs implements otlp.Sink.
func (p *Pipeline) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	now := p.opts.Now()
	rows := model.Logs(ld, now)
	return p.consume(kindLogs, len(rows), now, func() ([]byte, error) {
		return (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	}, func() { p.logs = append(p.logs, rows...) }, &p.nLogs)
}

// ConsumeTraces implements otlp.Sink.
func (p *Pipeline) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	rows := model.Spans(td)
	return p.consume(kindTraces, len(rows), p.opts.Now(), func() ([]byte, error) {
		return (&ptrace.ProtoMarshaler{}).MarshalTraces(td)
	}, func() { p.spans = append(p.spans, rows...) }, &p.nSpans)
}

// ConsumeMetrics implements otlp.Sink.
func (p *Pipeline) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	rows := model.MetricPoints(md)
	return p.consume(kindMetrics, len(rows), p.opts.Now(), func() ([]byte, error) {
		return (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
	}, func() { p.points = append(p.points, rows...) }, &p.nPoints)
}

func (p *Pipeline) consume(kind byte, n int, now time.Time, marshal func() ([]byte, error), buffer func(), counter *atomic.Int64) error {
	if n == 0 {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.bufMu.Lock()
	buffered := p.bufferedLocked(kind)
	p.bufMu.Unlock()
	if buffered+n > p.opts.MaxBufferedRows {
		p.requestFlush()
		return fmt.Errorf("%w: %d rows buffered", otlp.ErrBackpressure, buffered)
	}

	body, err := marshal()
	if err != nil {
		return fmt.Errorf("ingest: encode: %w", err)
	}
	if err := p.wal.Append(walRecord(kind, now, body)); err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	p.bufMu.Lock()
	buffer()
	full := p.bufferedLocked(kind) >= p.opts.MaxBufferedRows/2
	p.bufMu.Unlock()
	counter.Add(int64(n))
	if full {
		p.requestFlush()
	}
	return nil
}

func (p *Pipeline) bufferedLocked(kind byte) int {
	switch kind {
	case kindLogs:
		return len(p.logs)
	case kindTraces:
		return len(p.spans)
	default:
		return len(p.points)
	}
}

func (p *Pipeline) requestFlush() {
	select {
	case p.flushCh <- struct{}{}:
	default:
	}
}

// Run flushes periodically, or when buffers fill up, until ctx is done.
// It flushes one last time before returning.
func (p *Pipeline) Run(ctx context.Context) error {
	t := time.NewTicker(p.opts.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return p.Flush(context.WithoutCancel(ctx))
		case <-t.C:
		case <-p.flushCh:
		}
		if err := p.Flush(ctx); err != nil {
			p.opts.Logger.Error("flush failed, will retry", "err", err)
		}
	}
}

// Flush persists every buffered row. On failure, rows that could not be
// written stay buffered and the WAL is kept, so a later Flush retries.
func (p *Pipeline) Flush(ctx context.Context) error {
	p.flushMu.Lock()
	defer p.flushMu.Unlock()

	p.mu.Lock()
	seal, err := p.wal.Rotate()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("ingest: %w", err)
	}
	p.bufMu.Lock()
	logs, spans, points := p.logs, p.spans, p.points
	p.logs, p.spans, p.points = nil, nil, nil
	p.bufMu.Unlock()
	p.mu.Unlock()

	model.SortLogs(logs)
	model.SortSpans(spans)
	model.SortMetricPoints(points)

	failedLogs, errLogs := writeByHour(ctx, p, layout.Logs, logs, model.LogTime)
	failedSpans, errSpans := writeByHour(ctx, p, layout.Spans, spans, model.SpanTime)
	failedPoints, errPoints := writeByHour(ctx, p, layout.MetricPoints, points, model.PointTime)

	if err := errors.Join(errLogs, errSpans, errPoints); err != nil {
		p.bufMu.Lock()
		p.logs = append(failedLogs, p.logs...)
		p.spans = append(failedSpans, p.spans...)
		p.points = append(failedPoints, p.points...)
		p.bufMu.Unlock()
		return err
	}
	if err := p.wal.Remove(seal); err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	return nil
}

// writeByHour writes one Parquet file per hour present in rows (which keep
// their order) and returns the rows it failed to write.
func writeByHour[T any](ctx context.Context, p *Pipeline, dir string, rows []T, at func(T) int64) ([]T, error) {
	groups := map[string][]T{}
	hours := map[string]time.Time{}
	var order []string
	for _, r := range rows {
		t := time.Unix(0, at(r))
		part := layout.Partition(t)
		if _, ok := groups[part]; !ok {
			order = append(order, part)
			hours[part] = t
		}
		groups[part] = append(groups[part], r)
	}
	var failed []T
	var errs []error
	for _, part := range order {
		key := layout.NewKey(dir, hours[part])
		var buf bytes.Buffer
		err := parquet.Write(&buf, groups[part])
		if err == nil {
			err = p.opts.Store.Put(ctx, key, &buf)
		}
		if err != nil {
			failed = append(failed, groups[part]...)
			errs = append(errs, fmt.Errorf("ingest: write %s: %w", key, err))
			continue
		}
		p.opts.Logger.Debug("flushed", "key", key, "rows", len(groups[part]))
	}
	return failed, errors.Join(errs...)
}

// Stats returns the number of items received since start.
func (p *Pipeline) Stats() Stats {
	return Stats{Spans: p.nSpans.Load(), DataPoints: p.nPoints.Load(), LogRecords: p.nLogs.Load()}
}

// Close closes the WAL without flushing. Unflushed data is replayed on the
// next start.
func (p *Pipeline) Close() error {
	return p.wal.Close()
}

// walRecord frames a batch: kind | received-at (unix nanos) | OTLP protobuf.
func walRecord(kind byte, receivedAt time.Time, body []byte) []byte {
	rec := make([]byte, 9, 9+len(body))
	rec[0] = kind
	binary.LittleEndian.PutUint64(rec[1:9], uint64(receivedAt.UnixNano())) //nolint:gosec // timestamps are positive
	return append(rec, body...)
}

func (p *Pipeline) replay(rec []byte) error {
	if len(rec) < 9 {
		return nil
	}
	kind, body := rec[0], rec[9:]
	receivedAt := time.Unix(0, int64(binary.LittleEndian.Uint64(rec[1:9]))) //nolint:gosec // written by walRecord
	switch kind {
	case kindLogs:
		ld, err := (&plog.ProtoUnmarshaler{}).UnmarshalLogs(body)
		if err != nil {
			return err
		}
		p.logs = append(p.logs, model.Logs(ld, receivedAt)...)
	case kindTraces:
		td, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(body)
		if err != nil {
			return err
		}
		p.spans = append(p.spans, model.Spans(td)...)
	case kindMetrics:
		md, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(body)
		if err != nil {
			return err
		}
		p.points = append(p.points, model.MetricPoints(md)...)
	}
	return nil
}
