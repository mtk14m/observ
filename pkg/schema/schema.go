// Package schema defines the public, versioned storage schema of obsrv.
//
// Each type is one row of a Parquet file. Column names follow the
// OpenTelemetry data model so that the files can be read by any tool
// (DuckDB, Spark, …) without obsrv. Changes must be backward compatible;
// breaking changes require a new major version directory (v2/).
//
// Version 1 is a draft until obsrv 1.0.
package schema

// Version is the storage schema version, used as the top-level key prefix.
const Version = "v1"

// Log is one OpenTelemetry log record.
type Log struct {
	TimeUnixNano         int64             `parquet:"time_unix_nano"`
	ObservedTimeUnixNano int64             `parquet:"observed_time_unix_nano"`
	ServiceName          string            `parquet:"service_name,dict"`
	SeverityNumber       int32             `parquet:"severity_number"`
	SeverityText         string            `parquet:"severity_text,dict"`
	Body                 string            `parquet:"body"`
	TraceID              string            `parquet:"trace_id"`
	SpanID               string            `parquet:"span_id"`
	ScopeName            string            `parquet:"scope_name,dict"`
	Attributes           map[string]string `parquet:"attributes"`
	ResourceAttributes   map[string]string `parquet:"resource_attributes"`
}

// Span is one OpenTelemetry span.
type Span struct {
	TraceID            string            `parquet:"trace_id"`
	SpanID             string            `parquet:"span_id"`
	ParentSpanID       string            `parquet:"parent_span_id"`
	Name               string            `parquet:"name,dict"`
	Kind               string            `parquet:"kind,dict"`
	StartTimeUnixNano  int64             `parquet:"start_time_unix_nano"`
	EndTimeUnixNano    int64             `parquet:"end_time_unix_nano"`
	DurationNano       int64             `parquet:"duration_nano"`
	StatusCode         string            `parquet:"status_code,dict"`
	StatusMessage      string            `parquet:"status_message"`
	ServiceName        string            `parquet:"service_name,dict"`
	ScopeName          string            `parquet:"scope_name,dict"`
	Attributes         map[string]string `parquet:"attributes"`
	ResourceAttributes map[string]string `parquet:"resource_attributes"`
	Events             []SpanEvent       `parquet:"events"`
}

// SpanEvent is an event recorded on a span, such as an exception.
type SpanEvent struct {
	TimeUnixNano int64             `parquet:"time_unix_nano"`
	Name         string            `parquet:"name"`
	Attributes   map[string]string `parquet:"attributes"`
}

// Metric types, as named by the OpenTelemetry data model.
const (
	MetricGauge                = "Gauge"
	MetricSum                  = "Sum"
	MetricHistogram            = "Histogram"
	MetricExponentialHistogram = "ExponentialHistogram"
	MetricSummary              = "Summary"
)

// Aggregation temporalities.
const (
	TemporalityCumulative = "Cumulative"
	TemporalityDelta      = "Delta"
)

// MetricPoint is one data point of an OpenTelemetry metric.
//
// Gauge and Sum points use Value. Histogram, ExponentialHistogram and
// Summary points use Count and Sum; explicit-bucket histograms also fill
// ExplicitBounds and BucketCounts.
type MetricPoint struct {
	MetricName         string            `parquet:"metric_name,dict"`
	Type               string            `parquet:"type,dict"`
	Unit               string            `parquet:"unit,dict"`
	Temporality        string            `parquet:"temporality,dict"`
	IsMonotonic        bool              `parquet:"is_monotonic"`
	SeriesID           int64             `parquet:"series_id"`
	ServiceName        string            `parquet:"service_name,dict"`
	ScopeName          string            `parquet:"scope_name,dict"`
	Attributes         map[string]string `parquet:"attributes"`
	ResourceAttributes map[string]string `parquet:"resource_attributes"`
	StartTimeUnixNano  int64             `parquet:"start_time_unix_nano"`
	TimeUnixNano       int64             `parquet:"time_unix_nano"`
	Value              float64           `parquet:"value"`
	Count              int64             `parquet:"count"`
	Sum                float64           `parquet:"sum"`
	Min                *float64          `parquet:"min,optional"`
	Max                *float64          `parquet:"max,optional"`
	ExplicitBounds     []float64         `parquet:"explicit_bounds,list"`
	BucketCounts       []int64           `parquet:"bucket_counts,list"`
}
