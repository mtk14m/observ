package ingest_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/otlp"
	"github.com/mtk14n/obsrv/pkg/schema"
)

var _ otlp.Sink = (*ingest.Pipeline)(nil)

var base = time.Date(2026, 10, 7, 13, 58, 0, 0, time.UTC)

type env struct {
	walDir string
	store  objstore.ObjectStore
}

func newEnv(t *testing.T) env {
	t.Helper()
	store, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return env{walDir: t.TempDir(), store: store}
}

func (e env) pipeline(t *testing.T, mutate ...func(*ingest.Options)) *ingest.Pipeline {
	t.Helper()
	opts := ingest.Options{WALDir: e.walDir, Store: e.store, Now: func() time.Time { return base }}
	for _, m := range mutate {
		m(&opts)
	}
	p, err := ingest.New(opts)
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func logsAt(service string, times ...time.Time) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", service)
	recs := rl.ScopeLogs().AppendEmpty().LogRecords()
	for _, ts := range times {
		lr := recs.AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		lr.Body().SetStr(service + "@" + ts.Format("15:04"))
	}
	return ld
}

func spansAt(times ...time.Time) ptrace.Traces {
	td := ptrace.NewTraces()
	spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	for i, ts := range times {
		s := spans.AppendEmpty()
		s.SetTraceID(pcommon.TraceID{byte(len(times) - i)})
		s.SetSpanID(pcommon.SpanID{byte(i + 1)})
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(ts))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(ts.Add(time.Millisecond)))
	}
	return td
}

func gaugeAt(times ...time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("queue.size")
	dps := m.SetEmptyGauge().DataPoints()
	for _, ts := range times {
		dps.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}
	return md
}

func keys(t *testing.T, store objstore.ObjectStore, prefix string) []string {
	t.Helper()
	infos, err := store.List(context.Background(), prefix)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, info := range infos {
		out = append(out, info.Key)
	}
	return out
}

func readAll[T any](t *testing.T, store objstore.ObjectStore, prefix string) []T {
	t.Helper()
	var rows []T
	for _, key := range keys(t, store, prefix) {
		rc, err := store.Get(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		got, err := parquet.Read[T](bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		rows = append(rows, got...)
	}
	return rows
}

func TestFlushWritesEverySignalToItsPartition(t *testing.T) {
	e := newEnv(t)
	p := e.pipeline(t)
	ctx := t.Context()

	if err := p.ConsumeLogs(ctx, logsAt("api", base)); err != nil {
		t.Fatal(err)
	}
	if err := p.ConsumeTraces(ctx, spansAt(base)); err != nil {
		t.Fatal(err)
	}
	if err := p.ConsumeMetrics(ctx, gaugeAt(base)); err != nil {
		t.Fatal(err)
	}
	if err := p.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	for _, prefix := range []string{
		"v1/logs/date=2026-10-07/hour=13/",
		"v1/spans/date=2026-10-07/hour=13/",
		"v1/metric_points/date=2026-10-07/hour=13/",
	} {
		got := keys(t, e.store, prefix)
		if len(got) != 1 || !strings.HasSuffix(got[0], ".parquet") {
			t.Errorf("files under %s = %v, want one .parquet file", prefix, got)
		}
	}
}

func TestRowsAreSplitByHourSoPartitionsCanBePruned(t *testing.T) {
	e := newEnv(t)
	p := e.pipeline(t)
	later := base.Add(5 * time.Minute) // 14:03, next hour
	if err := p.ConsumeLogs(t.Context(), logsAt("api", base, later)); err != nil {
		t.Fatal(err)
	}
	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := len(keys(t, e.store, "v1/logs/date=2026-10-07/hour=13/")); n != 1 {
		t.Errorf("hour 13 has %d files, want 1", n)
	}
	if n := len(keys(t, e.store, "v1/logs/date=2026-10-07/hour=14/")); n != 1 {
		t.Errorf("hour 14 has %d files, want 1", n)
	}
}

func TestRowsAreSortedForCompression(t *testing.T) {
	e := newEnv(t)
	p := e.pipeline(t)
	ctx := t.Context()
	_ = p.ConsumeLogs(ctx, logsAt("web", base.Add(2*time.Second), base.Add(time.Second)))
	_ = p.ConsumeLogs(ctx, logsAt("api", base.Add(3*time.Second)))
	_ = p.ConsumeTraces(ctx, spansAt(base, base.Add(time.Second))) // trace IDs in descending order
	if err := p.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	logs := readAll[schema.Log](t, e.store, "v1/logs/")
	var order []string
	for _, l := range logs {
		order = append(order, l.Body)
	}
	if want := []string{"api@13:58", "web@13:58", "web@13:58"}; !slices.Equal(order, want) ||
		logs[1].TimeUnixNano > logs[2].TimeUnixNano {
		t.Errorf("logs not sorted by (service, time): %v", order)
	}

	spans := readAll[schema.Span](t, e.store, "v1/spans/")
	if len(spans) != 2 || spans[0].TraceID > spans[1].TraceID {
		t.Errorf("spans not sorted by trace_id: %v, %v", spans[0].TraceID, spans[1].TraceID)
	}
}

func TestUnflushedDataSurvivesACrash(t *testing.T) {
	e := newEnv(t)
	crashed := e.pipeline(t)
	if err := crashed.ConsumeLogs(t.Context(), logsAt("api", base, base)); err != nil {
		t.Fatal(err)
	}
	// No flush: the process dies here. Release the files as the OS would.
	_ = crashed.Close()

	restarted := e.pipeline(t)
	if err := restarted.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := readAll[schema.Log](t, e.store, "v1/logs/"); len(got) != 2 {
		t.Errorf("recovered %d log records, want 2", len(got))
	}
}

func TestFlushedDataIsNotReplayedAfterRestart(t *testing.T) {
	e := newEnv(t)
	first := e.pipeline(t)
	_ = first.ConsumeLogs(t.Context(), logsAt("api", base))
	if err := first.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	_ = first.Close()

	second := e.pipeline(t)
	if err := second.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := readAll[schema.Log](t, e.store, "v1/logs/"); len(got) != 1 {
		t.Errorf("found %d log records after restart, want 1 (no duplicates)", len(got))
	}
}

// flakyStore fails Put while broken is true.
type flakyStore struct {
	objstore.ObjectStore
	broken atomic.Bool
}

func (s *flakyStore) Put(ctx context.Context, key string, r io.Reader) error {
	if s.broken.Load() {
		return errors.New("bucket unavailable")
	}
	return s.ObjectStore.Put(ctx, key, r)
}

func TestFailedFlushKeepsDataAndRetries(t *testing.T) {
	e := newEnv(t)
	store := &flakyStore{ObjectStore: e.store}
	store.broken.Store(true)
	p := e.pipeline(t, func(o *ingest.Options) { o.Store = store })

	_ = p.ConsumeLogs(t.Context(), logsAt("api", base))
	if err := p.Flush(t.Context()); err == nil {
		t.Fatal("Flush with a broken store returned nil error")
	}

	store.broken.Store(false)
	if err := p.Flush(t.Context()); err != nil {
		t.Fatalf("retry Flush: %v", err)
	}
	if got := readAll[schema.Log](t, e.store, "v1/logs/"); len(got) != 1 {
		t.Errorf("found %d log records, want 1", len(got))
	}
}

func TestBackpressureWhenBufferIsFull(t *testing.T) {
	e := newEnv(t)
	p := e.pipeline(t, func(o *ingest.Options) { o.MaxBufferedRows = 2 })

	if err := p.ConsumeLogs(t.Context(), logsAt("api", base, base)); err != nil {
		t.Fatalf("first batch: %v", err)
	}
	err := p.ConsumeLogs(t.Context(), logsAt("api", base))
	if !errors.Is(err, otlp.ErrBackpressure) {
		t.Errorf("ConsumeLogs on a full buffer = %v, want ErrBackpressure", err)
	}
}

func TestRunFlushesPeriodicallyAndOnShutdown(t *testing.T) {
	e := newEnv(t)
	p := e.pipeline(t, func(o *ingest.Options) { o.FlushInterval = 20 * time.Millisecond })
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	_ = p.ConsumeLogs(t.Context(), logsAt("api", base))
	deadline := time.Now().Add(2 * time.Second)
	for len(keys(t, e.store, "v1/logs/")) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("periodic flush did not happen")
		}
		time.Sleep(5 * time.Millisecond)
	}

	_ = p.ConsumeLogs(t.Context(), logsAt("web", base))
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := readAll[schema.Log](t, e.store, "v1/logs/"); len(got) != 2 {
		t.Errorf("found %d log records after shutdown, want 2", len(got))
	}
}

func TestStats(t *testing.T) {
	p := newEnv(t).pipeline(t)
	_ = p.ConsumeLogs(t.Context(), logsAt("api", base, base))
	_ = p.ConsumeTraces(t.Context(), spansAt(base))
	_ = p.ConsumeMetrics(t.Context(), gaugeAt(base, base, base))

	want := ingest.Stats{Spans: 1, DataPoints: 3, LogRecords: 2}
	if got := p.Stats(); got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}
}
