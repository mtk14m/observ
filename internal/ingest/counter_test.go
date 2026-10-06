package ingest_test

import (
	"sync"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/otlp"
)

var _ otlp.Sink = (*ingest.Counter)(nil)

func TestCounterCountsEverySignalConcurrently(t *testing.T) {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().
		SetEmptyGauge().DataPoints().AppendEmpty()

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	c := &ingest.Counter{}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = c.ConsumeTraces(t.Context(), td)
			_ = c.ConsumeMetrics(t.Context(), md)
			_ = c.ConsumeLogs(t.Context(), ld)
		})
	}
	wg.Wait()

	want := ingest.Stats{Spans: 50, DataPoints: 50, LogRecords: 50}
	if got := c.Stats(); got != want {
		t.Errorf("Stats() = %+v, want %+v", got, want)
	}
}
