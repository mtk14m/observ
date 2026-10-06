package model_test

import (
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/model"
	"github.com/mtk14n/obsrv/pkg/schema"
)

var (
	t0      = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	traceID = pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	parent  = pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1}
)

const (
	traceHex  = "0102030405060708090a0b0c0d0e0f10"
	spanHex   = "0102030405060708"
	parentHex = "0807060504030201"
)

func ts(d time.Duration) pcommon.Timestamp { return pcommon.NewTimestampFromTime(t0.Add(d)) }

func resource(r pcommon.Resource) {
	r.Attributes().PutStr("service.name", "checkout")
	r.Attributes().PutStr("deployment.environment.name", "prod")
}

func TestLogs(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	resource(rl.Resource())
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("app/logger")

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(ts(time.Second))
	lr.SetObservedTimestamp(ts(2 * time.Second))
	lr.SetSeverityNumber(plog.SeverityNumberError)
	lr.SetSeverityText("ERROR")
	lr.Body().SetStr("payment failed")
	lr.SetTraceID(traceID)
	lr.SetSpanID(spanID)
	lr.Attributes().PutInt("http.response.status_code", 502)
	lr.Attributes().PutBool("retry", true)

	// No timestamp, no severity text, structured body.
	lr2 := sl.LogRecords().AppendEmpty()
	lr2.SetObservedTimestamp(ts(3 * time.Second))
	lr2.SetSeverityNumber(plog.SeverityNumberWarn2)
	lr2.Body().SetEmptyMap().PutStr("k", "v")

	got := model.Logs(ld, t0)
	want := []schema.Log{
		{
			TimeUnixNano:         t0.Add(time.Second).UnixNano(),
			ObservedTimeUnixNano: t0.Add(2 * time.Second).UnixNano(),
			ServiceName:          "checkout",
			SeverityNumber:       int32(plog.SeverityNumberError),
			SeverityText:         "ERROR",
			Body:                 "payment failed",
			TraceID:              traceHex,
			SpanID:               spanHex,
			ScopeName:            "app/logger",
			Attributes:           map[string]string{"http.response.status_code": "502", "retry": "true"},
			ResourceAttributes:   map[string]string{"service.name": "checkout", "deployment.environment.name": "prod"},
		},
		{
			TimeUnixNano:         t0.Add(3 * time.Second).UnixNano(),
			ObservedTimeUnixNano: t0.Add(3 * time.Second).UnixNano(),
			ServiceName:          "checkout",
			SeverityNumber:       int32(plog.SeverityNumberWarn2),
			SeverityText:         "WARN",
			Body:                 `{"k":"v"}`,
			ScopeName:            "app/logger",
			Attributes:           map[string]string{},
			ResourceAttributes:   map[string]string{"service.name": "checkout", "deployment.environment.name": "prod"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Logs() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLogsWithoutAnyTimestampUseReceiveTime(t *testing.T) {
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	got := model.Logs(ld, t0)
	if got[0].TimeUnixNano != t0.UnixNano() || got[0].ObservedTimeUnixNano != t0.UnixNano() {
		t.Errorf("times = %d/%d, want %d", got[0].TimeUnixNano, got[0].ObservedTimeUnixNano, t0.UnixNano())
	}
	if got[0].ServiceName != "unknown_service" {
		t.Errorf("ServiceName = %q, want unknown_service", got[0].ServiceName)
	}
}

func TestSpans(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	resource(rs.Resource())
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("otelhttp")

	s := ss.Spans().AppendEmpty()
	s.SetTraceID(traceID)
	s.SetSpanID(spanID)
	s.SetParentSpanID(parent)
	s.SetName("POST /pay")
	s.SetKind(ptrace.SpanKindServer)
	s.SetStartTimestamp(ts(0))
	s.SetEndTimestamp(ts(150 * time.Millisecond))
	s.Status().SetCode(ptrace.StatusCodeError)
	s.Status().SetMessage("card declined")
	s.Attributes().PutStr("http.route", "/pay")
	ev := s.Events().AppendEmpty()
	ev.SetTimestamp(ts(100 * time.Millisecond))
	ev.SetName("exception")
	ev.Attributes().PutStr("exception.type", "CardDeclined")

	got := model.Spans(td)
	want := []schema.Span{{
		TraceID:            traceHex,
		SpanID:             spanHex,
		ParentSpanID:       parentHex,
		Name:               "POST /pay",
		Kind:               "Server",
		StartTimeUnixNano:  t0.UnixNano(),
		EndTimeUnixNano:    t0.Add(150 * time.Millisecond).UnixNano(),
		DurationNano:       (150 * time.Millisecond).Nanoseconds(),
		StatusCode:         "Error",
		StatusMessage:      "card declined",
		ServiceName:        "checkout",
		ScopeName:          "otelhttp",
		Attributes:         map[string]string{"http.route": "/pay"},
		ResourceAttributes: map[string]string{"service.name": "checkout", "deployment.environment.name": "prod"},
		Events: []schema.SpanEvent{{
			TimeUnixNano: t0.Add(100 * time.Millisecond).UnixNano(),
			Name:         "exception",
			Attributes:   map[string]string{"exception.type": "CardDeclined"},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spans() =\n%+v\nwant\n%+v", got, want)
	}
}

func newMetric(md pmetric.Metrics, name string) pmetric.Metric {
	rm := md.ResourceMetrics().AppendEmpty()
	resource(rm.Resource())
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(name)
	m.SetUnit("1")
	return m
}

func TestMetricGaugeAndSum(t *testing.T) {
	md := pmetric.NewMetrics()
	g := newMetric(md, "queue.size").SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts(time.Second))
	dp.SetIntValue(42)
	dp.Attributes().PutStr("queue", "orders")

	sum := newMetric(md, "orders.count").SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)
	sp := sum.DataPoints().AppendEmpty()
	sp.SetStartTimestamp(ts(0))
	sp.SetTimestamp(ts(time.Second))
	sp.SetDoubleValue(7.5)

	got := model.MetricPoints(md)
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2", len(got))
	}

	gp := got[0]
	if gp.MetricName != "queue.size" || gp.Type != schema.MetricGauge || gp.Value != 42 ||
		gp.Unit != "1" || gp.ServiceName != "checkout" || gp.Attributes["queue"] != "orders" ||
		gp.TimeUnixNano != t0.Add(time.Second).UnixNano() || gp.Temporality != "" {
		t.Errorf("gauge point = %+v", gp)
	}

	sp2 := got[1]
	if sp2.Type != schema.MetricSum || sp2.Value != 7.5 || !sp2.IsMonotonic ||
		sp2.Temporality != schema.TemporalityCumulative || sp2.StartTimeUnixNano != t0.UnixNano() {
		t.Errorf("sum point = %+v", sp2)
	}
	if gp.SeriesID == 0 || gp.SeriesID == sp2.SeriesID {
		t.Errorf("series IDs must be non-zero and distinct: %d, %d", gp.SeriesID, sp2.SeriesID)
	}
}

func TestMetricHistogram(t *testing.T) {
	md := pmetric.NewMetrics()
	h := newMetric(md, "http.server.request.duration").SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := h.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts(time.Second))
	dp.SetCount(10)
	dp.SetSum(2.5)
	dp.SetMin(0.01)
	dp.SetMax(1.2)
	dp.ExplicitBounds().FromRaw([]float64{0.1, 0.5})
	dp.BucketCounts().FromRaw([]uint64{6, 3, 1})

	got := model.MetricPoints(md)[0]
	if got.Type != schema.MetricHistogram || got.Count != 10 || got.Sum != 2.5 ||
		got.Temporality != schema.TemporalityDelta ||
		got.Min == nil || *got.Min != 0.01 || got.Max == nil || *got.Max != 1.2 ||
		!reflect.DeepEqual(got.ExplicitBounds, []float64{0.1, 0.5}) ||
		!reflect.DeepEqual(got.BucketCounts, []int64{6, 3, 1}) {
		t.Errorf("histogram point = %+v", got)
	}
}

func TestSeriesIDIsStableAndIgnoresAttributeOrder(t *testing.T) {
	build := func(keys ...string) int64 {
		md := pmetric.NewMetrics()
		dp := newMetric(md, "m").SetEmptyGauge().DataPoints().AppendEmpty()
		for _, k := range keys {
			dp.Attributes().PutStr(k, "v")
		}
		return model.MetricPoints(md)[0].SeriesID
	}
	if a, b := build("a", "b"), build("b", "a"); a != b {
		t.Errorf("series ID depends on attribute order: %d != %d", a, b)
	}
	if a, b := build("a"), build("b"); a == b {
		t.Errorf("different attributes produced the same series ID %d", a)
	}
}
