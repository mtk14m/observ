// Package model converts OTLP data (pdata) into rows of the public storage
// schema (pkg/schema).
package model

import (
	"hash/fnv"
	"maps"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/pkg/schema"
)

// unknownService is the OpenTelemetry default when service.name is unset.
const unknownService = "unknown_service"

// Logs converts log records. receivedAt is used when a record has neither a
// timestamp nor an observed timestamp.
func Logs(ld plog.Logs, receivedAt time.Time) []schema.Log {
	rows := make([]schema.Log, 0, ld.LogRecordCount())
	for _, rl := range ld.ResourceLogs().All() {
		res := attrs(rl.Resource().Attributes())
		service := serviceName(res)
		for _, sl := range rl.ScopeLogs().All() {
			scope := sl.Scope().Name()
			for _, lr := range sl.LogRecords().All() {
				observed := int64(lr.ObservedTimestamp())
				if observed == 0 {
					observed = receivedAt.UnixNano()
				}
				t := int64(lr.Timestamp())
				if t == 0 {
					t = observed
				}
				text := lr.SeverityText()
				if text == "" {
					text = severityText(lr.SeverityNumber())
				}
				rows = append(rows, schema.Log{
					TimeUnixNano:         t,
					ObservedTimeUnixNano: observed,
					ServiceName:          service,
					SeverityNumber:       int32(lr.SeverityNumber()),
					SeverityText:         text,
					Body:                 lr.Body().AsString(),
					TraceID:              traceID(lr.TraceID()),
					SpanID:               spanID(lr.SpanID()),
					ScopeName:            scope,
					Attributes:           attrs(lr.Attributes()),
					ResourceAttributes:   res,
				})
			}
		}
	}
	return rows
}

// Spans converts spans.
func Spans(td ptrace.Traces) []schema.Span {
	rows := make([]schema.Span, 0, td.SpanCount())
	for _, rs := range td.ResourceSpans().All() {
		res := attrs(rs.Resource().Attributes())
		service := serviceName(res)
		for _, ss := range rs.ScopeSpans().All() {
			scope := ss.Scope().Name()
			for _, s := range ss.Spans().All() {
				var events []schema.SpanEvent
				for _, ev := range s.Events().All() {
					events = append(events, schema.SpanEvent{
						TimeUnixNano: int64(ev.Timestamp()),
						Name:         ev.Name(),
						Attributes:   attrs(ev.Attributes()),
					})
				}
				start, end := int64(s.StartTimestamp()), int64(s.EndTimestamp())
				rows = append(rows, schema.Span{
					TraceID:            traceID(s.TraceID()),
					SpanID:             spanID(s.SpanID()),
					ParentSpanID:       spanID(s.ParentSpanID()),
					Name:               s.Name(),
					Kind:               s.Kind().String(),
					StartTimeUnixNano:  start,
					EndTimeUnixNano:    end,
					DurationNano:       max(end-start, 0),
					StatusCode:         s.Status().Code().String(),
					StatusMessage:      s.Status().Message(),
					ServiceName:        service,
					ScopeName:          scope,
					Attributes:         attrs(s.Attributes()),
					ResourceAttributes: res,
					Events:             events,
				})
			}
		}
	}
	return rows
}

// MetricPoints converts metrics into one row per data point.
func MetricPoints(md pmetric.Metrics) []schema.MetricPoint {
	rows := make([]schema.MetricPoint, 0, md.DataPointCount())
	for _, rm := range md.ResourceMetrics().All() {
		res := attrs(rm.Resource().Attributes())
		service := serviceName(res)
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				base := schema.MetricPoint{
					MetricName:         m.Name(),
					Type:               m.Type().String(),
					Unit:               m.Unit(),
					ServiceName:        service,
					ScopeName:          sm.Scope().Name(),
					ResourceAttributes: res,
				}
				rows = appendPoints(rows, base, m)
			}
		}
	}
	return rows
}

func appendPoints(rows []schema.MetricPoint, base schema.MetricPoint, m pmetric.Metric) []schema.MetricPoint {
	point := func(a pcommon.Map, start, t pcommon.Timestamp) schema.MetricPoint {
		p := base
		p.Attributes = attrs(a)
		p.StartTimeUnixNano = int64(start)
		p.TimeUnixNano = int64(t)
		p.SeriesID = seriesID(p)
		return p
	}
	numberValue := func(dp pmetric.NumberDataPoint) float64 {
		if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
			return float64(dp.IntValue())
		}
		return dp.DoubleValue()
	}

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for _, dp := range m.Gauge().DataPoints().All() {
			p := point(dp.Attributes(), dp.StartTimestamp(), dp.Timestamp())
			p.Value = numberValue(dp)
			rows = append(rows, p)
		}
	case pmetric.MetricTypeSum:
		base.Temporality = m.Sum().AggregationTemporality().String()
		base.IsMonotonic = m.Sum().IsMonotonic()
		for _, dp := range m.Sum().DataPoints().All() {
			p := point(dp.Attributes(), dp.StartTimestamp(), dp.Timestamp())
			p.Value = numberValue(dp)
			rows = append(rows, p)
		}
	case pmetric.MetricTypeHistogram:
		base.Temporality = m.Histogram().AggregationTemporality().String()
		for _, dp := range m.Histogram().DataPoints().All() {
			p := point(dp.Attributes(), dp.StartTimestamp(), dp.Timestamp())
			p.Count, p.Sum = int64(dp.Count()), dp.Sum()
			if dp.HasMin() {
				p.Min = new(dp.Min())
			}
			if dp.HasMax() {
				p.Max = new(dp.Max())
			}
			p.ExplicitBounds = dp.ExplicitBounds().AsRaw()
			for _, c := range dp.BucketCounts().All() {
				p.BucketCounts = append(p.BucketCounts, int64(c))
			}
			rows = append(rows, p)
		}
	case pmetric.MetricTypeExponentialHistogram:
		base.Temporality = m.ExponentialHistogram().AggregationTemporality().String()
		for _, dp := range m.ExponentialHistogram().DataPoints().All() {
			p := point(dp.Attributes(), dp.StartTimestamp(), dp.Timestamp())
			p.Count, p.Sum = int64(dp.Count()), dp.Sum()
			if dp.HasMin() {
				p.Min = new(dp.Min())
			}
			if dp.HasMax() {
				p.Max = new(dp.Max())
			}
			rows = append(rows, p)
		}
	case pmetric.MetricTypeSummary:
		for _, dp := range m.Summary().DataPoints().All() {
			p := point(dp.Attributes(), dp.StartTimestamp(), dp.Timestamp())
			p.Count, p.Sum = int64(dp.Count()), dp.Sum()
			rows = append(rows, p)
		}
	case pmetric.MetricTypeEmpty:
	}
	return rows
}

// seriesID identifies a time series: metric name, type and every attribute.
// It is stable across processes and independent of attribute order.
func seriesID(p schema.MetricPoint) int64 {
	h := fnv.New64a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	writeMap := func(m map[string]string) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			write(k)
			write(m[k])
		}
		write("")
	}
	write(p.MetricName)
	write(p.Type)
	writeMap(p.Attributes)
	writeMap(p.ResourceAttributes)
	return int64(h.Sum64()) //nolint:gosec // a hash, wrapping is intended
}

func attrs(m pcommon.Map) map[string]string {
	out := make(map[string]string, m.Len())
	for k, v := range m.All() {
		out[k] = v.AsString()
	}
	return out
}

func serviceName(res map[string]string) string {
	if s := res["service.name"]; s != "" {
		return s
	}
	return unknownService
}

func traceID(id pcommon.TraceID) string {
	if id.IsEmpty() {
		return ""
	}
	return id.String()
}

func spanID(id pcommon.SpanID) string {
	if id.IsEmpty() {
		return ""
	}
	return id.String()
}

// severityText derives a text from the severity number ranges defined by
// the OpenTelemetry log data model.
func severityText(n plog.SeverityNumber) string {
	switch {
	case n >= plog.SeverityNumberFatal:
		return "FATAL"
	case n >= plog.SeverityNumberError:
		return "ERROR"
	case n >= plog.SeverityNumberWarn:
		return "WARN"
	case n >= plog.SeverityNumberInfo:
		return "INFO"
	case n >= plog.SeverityNumberDebug:
		return "DEBUG"
	case n >= plog.SeverityNumberTrace:
		return "TRACE"
	default:
		return ""
	}
}
