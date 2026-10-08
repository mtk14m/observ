package query_test

import (
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/query"
)

var (
	base    = time.Date(2026, 10, 7, 13, 0, 0, 0, time.UTC)
	window  = query.TimeRange{From: base, To: base.Add(time.Hour)}
	trace1  = pcommon.TraceID{1}
	trace2  = pcommon.TraceID{2}
	trace1H = trace1.String()
	trace2H = trace2.String()
)

func at(d time.Duration) pcommon.Timestamp { return pcommon.NewTimestampFromTime(base.Add(d)) }

// fixture writes, through the real ingest pipeline:
//   - trace1 at +10m: frontend GET /checkout (200ms) → payment POST /pay (150ms, error, exception)
//   - trace2 at +20m: frontend GET /home (10ms)
//   - logs: frontend INFO "checkout started", payment ERROR "card declined" (both in trace1)
//   - metrics: gauge queue.size{queue=a|b}, counter orders (cumulative), histogram latency (delta)
func fixture(t *testing.T) *query.Engine {
	t.Helper()
	store, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	ctx := t.Context()

	td := ptrace.NewTraces()
	span := func(service string, tid pcommon.TraceID, sid, parent byte, name string, start, dur time.Duration, failed bool) {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", service)
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetTraceID(tid)
		s.SetSpanID(pcommon.SpanID{sid})
		if parent != 0 {
			s.SetParentSpanID(pcommon.SpanID{parent})
		}
		s.SetName(name)
		s.SetKind(ptrace.SpanKindServer)
		s.SetStartTimestamp(at(start))
		s.SetEndTimestamp(at(start + dur))
		s.Attributes().PutStr("http.route", name)
		if failed {
			s.Status().SetCode(ptrace.StatusCodeError)
			ev := s.Events().AppendEmpty()
			ev.SetName("exception")
			ev.Attributes().PutStr("exception.type", "CardDeclined")
		}
	}
	span("frontend", trace1, 1, 0, "GET /checkout", 10*time.Minute, 200*time.Millisecond, false)
	span("payment", trace1, 2, 1, "POST /pay", 10*time.Minute+20*time.Millisecond, 150*time.Millisecond, true)
	span("frontend", trace2, 3, 0, "GET /home", 20*time.Minute, 10*time.Millisecond, false)
	if err := p.ConsumeTraces(ctx, td); err != nil {
		t.Fatal(err)
	}

	ld := plog.NewLogs()
	logRec := func(service string, sev plog.SeverityNumber, body string, d time.Duration) {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", service)
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.SetTimestamp(at(d))
		lr.SetSeverityNumber(sev)
		lr.Body().SetStr(body)
		lr.SetTraceID(trace1)
		lr.Attributes().PutStr("order.id", "42")
	}
	logRec("frontend", plog.SeverityNumberInfo, "checkout started", 10*time.Minute)
	logRec("payment", plog.SeverityNumberError, "card declined", 10*time.Minute+time.Second)
	if err := p.ConsumeLogs(ctx, ld); err != nil {
		t.Fatal(err)
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "frontend")
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	g := ms.AppendEmpty()
	g.SetName("queue.size")
	g.SetUnit("{item}")
	gauge := g.SetEmptyGauge().DataPoints()
	for _, q := range []struct {
		name string
		v    int64
	}{{"a", 3}, {"b", 7}} {
		dp := gauge.AppendEmpty()
		dp.SetTimestamp(at(time.Minute))
		dp.SetIntValue(q.v)
		dp.Attributes().PutStr("queue", q.name)
	}

	c := ms.AppendEmpty()
	c.SetName("orders")
	sum := c.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for i, v := range []float64{10, 70} {
		dp := sum.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(at(0))
		dp.SetTimestamp(at(time.Duration(i+1) * time.Minute))
		dp.SetDoubleValue(v)
	}
	if err := p.ConsumeMetrics(ctx, md); err != nil {
		t.Fatal(err)
	}

	if err := p.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	e, err := query.New(store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestServices(t *testing.T) {
	got, err := fixture(t).Services(t.Context(), window)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d services, want 2: %+v", len(got), got)
	}
	fe, pay := got[0], got[1]
	if fe.Name != "frontend" || fe.Requests != 2 || fe.Errors != 0 {
		t.Errorf("frontend = %+v", fe)
	}
	if pay.Name != "payment" || pay.Requests != 1 || pay.Errors != 1 || pay.ErrorRate != 1 {
		t.Errorf("payment = %+v", pay)
	}
	if fe.P95Ms <= 0 || fe.P95Ms > 200 {
		t.Errorf("frontend p95 = %vms, want in (0, 200]", fe.P95Ms)
	}
}

func TestSearchLogs(t *testing.T) {
	e := fixture(t)

	all, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Body != "card declined" {
		t.Fatalf("logs = %+v, want 2, newest first", all)
	}
	if all[0].TraceID != trace1H || all[0].Attributes["order.id"] != "42" || all[0].Service != "payment" {
		t.Errorf("record = %+v", all[0])
	}

	errs, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window, Query: "level:error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || errs[0].Severity != "ERROR" {
		t.Errorf("level:error = %+v", errs)
	}

	byAttr, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window, Query: "42"})
	if err != nil || len(byAttr) != 2 {
		t.Errorf("free text matching an attribute value = %d records, %v; want 2", len(byAttr), err)
	}

	before := query.TimeRange{From: base, To: base.Add(5 * time.Minute)}
	if none, _ := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: before}); len(none) != 0 {
		t.Errorf("logs outside the range = %+v", none)
	}

	if _, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window, Query: `"bad`}); err == nil {
		t.Error("invalid query returned nil error")
	}
}

func TestLogHistogram(t *testing.T) {
	got, err := fixture(t).LogHistogram(t.Context(), query.LogQuery{TimeRange: window}, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].T != base.Add(10*time.Minute).UnixNano() ||
		got[0].Counts["INFO"] != 1 || got[0].Counts["ERROR"] != 1 {
		t.Errorf("histogram = %+v", got)
	}
}

func TestSearchTraces(t *testing.T) {
	e := fixture(t)
	all, err := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].TraceID != trace2H {
		t.Fatalf("traces = %+v, want 2, newest first", all)
	}
	t1 := all[1]
	if t1.RootService != "frontend" || t1.RootName != "GET /checkout" || t1.SpanCount != 2 ||
		t1.ErrorCount != 1 || t1.DurationNano != (200*time.Millisecond).Nanoseconds() ||
		!slices.Equal(t1.Services, []string{"frontend", "payment"}) {
		t.Errorf("trace1 summary = %+v", t1)
	}

	bySvc, _ := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window, Service: "payment"})
	if len(bySvc) != 1 || bySvc[0].TraceID != trace1H {
		t.Errorf("service:payment = %+v", bySvc)
	}
	errs, _ := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window, ErrorsOnly: true})
	if len(errs) != 1 || errs[0].TraceID != trace1H {
		t.Errorf("errors only = %+v", errs)
	}
	byAttr, err := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window, Query: `http.route:"POST /pay"`})
	if err != nil || len(byAttr) != 1 || byAttr[0].TraceID != trace1H {
		t.Errorf("traces with a span matching an attribute = %+v, %v", byAttr, err)
	}
	if _, err := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window, Query: "status:nope"}); err == nil {
		t.Error("invalid span query returned nil error")
	}
	slow, _ := e.SearchTraces(t.Context(), query.TraceQuery{TimeRange: window, MinDuration: 100 * time.Millisecond})
	if len(slow) != 1 || slow[0].TraceID != trace1H {
		t.Errorf("min duration = %+v", slow)
	}
}

func TestTrace(t *testing.T) {
	spans, err := fixture(t).Trace(t.Context(), trace1H, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 2 || spans[0].Name != "GET /checkout" || spans[1].ParentSpanID != spans[0].SpanID {
		t.Fatalf("spans = %+v", spans)
	}
	pay := spans[1]
	if pay.StatusCode != "Error" || len(pay.Events) != 1 || pay.Events[0].Attributes["exception.type"] != "CardDeclined" ||
		pay.Attributes["http.route"] != "POST /pay" || pay.ResourceAttributes["service.name"] != "payment" {
		t.Errorf("payment span = %+v", pay)
	}
}

func TestMetrics(t *testing.T) {
	got, err := fixture(t).Metrics(t.Context(), window)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "orders" || got[1].Name != "queue.size" {
		t.Fatalf("metrics = %+v", got)
	}
	if !got[0].Monotonic || got[1].Monotonic {
		t.Errorf("monotonic flags = %v, %v; want true for the counter only", got[0].Monotonic, got[1].Monotonic)
	}
	q := got[1]
	if q.Type != "Gauge" || q.Unit != "{item}" || q.Series != 2 || !slices.Contains(q.AttributeKeys, "queue") {
		t.Errorf("queue.size = %+v", q)
	}
}

func TestQueryMetric(t *testing.T) {
	e := fixture(t)
	gauge, err := e.QueryMetric(t.Context(), query.MetricQuery{
		TimeRange: window, Metric: "queue.size", GroupBy: []string{"queue"}, Step: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gauge) != 2 || gauge[0].Labels["queue"] != "a" || gauge[0].Points[0].V != 3 || gauge[1].Points[0].V != 7 {
		t.Errorf("gauge by queue = %+v", gauge)
	}

	filtered, _ := e.QueryMetric(t.Context(), query.MetricQuery{
		TimeRange: window, Metric: "queue.size", Filters: map[string]string{"queue": "b"}, Step: time.Minute,
	})
	if len(filtered) != 1 || filtered[0].Points[0].V != 7 {
		t.Errorf("filtered gauge = %+v", filtered)
	}

	rate, _ := e.QueryMetric(t.Context(), query.MetricQuery{TimeRange: window, Metric: "orders", Step: time.Minute})
	if len(rate) != 1 || len(rate[0].Points) != 1 || rate[0].Points[0].V != 1 { // +60 in 60s
		t.Errorf("counter rate = %+v", rate)
	}
}

func TestEmptyStoreReturnsNothing(t *testing.T) {
	store, _ := fs.New(t.TempDir())
	e, err := query.New(store)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Close() }()
	ctx := t.Context()
	if s, err := e.Services(ctx, window); err != nil || len(s) != 0 {
		t.Errorf("Services = %v, %v", s, err)
	}
	if l, err := e.SearchLogs(ctx, query.LogQuery{TimeRange: window}); err != nil || len(l) != 0 {
		t.Errorf("SearchLogs = %v, %v", l, err)
	}
	if tr, err := e.SearchTraces(ctx, query.TraceQuery{TimeRange: window}); err != nil || len(tr) != 0 {
		t.Errorf("SearchTraces = %v, %v", tr, err)
	}
	if m, err := e.Metrics(ctx, window); err != nil || len(m) != 0 {
		t.Errorf("Metrics = %v, %v", m, err)
	}
	if s, err := e.QueryMetric(ctx, query.MetricQuery{TimeRange: window, Metric: "x"}); err != nil || len(s) != 0 {
		t.Errorf("QueryMetric = %v, %v", s, err)
	}
}
