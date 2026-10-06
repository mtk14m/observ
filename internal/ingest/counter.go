// Package ingest turns received telemetry into stored telemetry.
package ingest

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Stats is a snapshot of the items received per signal.
type Stats struct {
	Spans      int64
	DataPoints int64
	LogRecords int64
}

// Counter is a Sink that only counts what it receives. It stands in for the
// storage pipeline until the WAL and Parquet writer land, and stays useful
// as a decorator for ingestion metrics.
type Counter struct {
	spans, points, logs atomic.Int64
}

// ConsumeTraces implements otlp.Sink.
func (c *Counter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	c.spans.Add(int64(td.SpanCount()))
	return nil
}

// ConsumeMetrics implements otlp.Sink.
func (c *Counter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	c.points.Add(int64(md.DataPointCount()))
	return nil
}

// ConsumeLogs implements otlp.Sink.
func (c *Counter) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	c.logs.Add(int64(ld.LogRecordCount()))
	return nil
}

// Stats returns the current totals.
func (c *Counter) Stats() Stats {
	return Stats{Spans: c.spans.Load(), DataPoints: c.points.Load(), LogRecords: c.logs.Load()}
}
