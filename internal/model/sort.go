package model

import (
	"cmp"
	"slices"

	"github.com/mtk14n/obsrv/pkg/schema"
)

// Rows are sorted before being written so that similar values sit together,
// which is what makes Parquet compression effective.

// SortLogs sorts logs by service, then time.
func SortLogs(rows []schema.Log) {
	slices.SortFunc(rows, func(a, b schema.Log) int {
		return cmp.Or(cmp.Compare(a.ServiceName, b.ServiceName), cmp.Compare(a.TimeUnixNano, b.TimeUnixNano))
	})
}

// SortSpans sorts spans by trace, then start time.
func SortSpans(rows []schema.Span) {
	slices.SortFunc(rows, func(a, b schema.Span) int {
		return cmp.Or(cmp.Compare(a.TraceID, b.TraceID), cmp.Compare(a.StartTimeUnixNano, b.StartTimeUnixNano))
	})
}

// SortMetricPoints sorts points by metric, series, then time.
func SortMetricPoints(rows []schema.MetricPoint) {
	slices.SortFunc(rows, func(a, b schema.MetricPoint) int {
		return cmp.Or(cmp.Compare(a.MetricName, b.MetricName), cmp.Compare(a.SeriesID, b.SeriesID),
			cmp.Compare(a.TimeUnixNano, b.TimeUnixNano))
	})
}

// LogTime returns the time of a log record, which decides its partition.
func LogTime(r schema.Log) int64 { return r.TimeUnixNano }

// SpanTime returns the start time of a span.
func SpanTime(r schema.Span) int64 { return r.StartTimeUnixNano }

// PointTime returns the time of a metric point.
func PointTime(r schema.MetricPoint) int64 { return r.TimeUnixNano }
