package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/api"
	"github.com/mtk14n/obsrv/internal/query"
	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

// fakeQuerier records the last call and returns canned results.
type fakeQuerier struct {
	last any
	err  error
}

func (f *fakeQuerier) Services(_ context.Context, r query.TimeRange) ([]query.ServiceSummary, error) {
	f.last = r
	return []query.ServiceSummary{{Name: "api", Requests: 3}}, f.err
}

func (f *fakeQuerier) ServiceDetail(_ context.Context, name string, r query.TimeRange, step time.Duration) (query.ServiceDetail, error) {
	f.last = []any{name, r, step}
	return query.ServiceDetail{Summary: query.ServiceSummary{Name: name, Requests: 1}}, f.err
}

func (f *fakeQuerier) ServiceMap(_ context.Context, r query.TimeRange) ([]query.Edge, error) {
	f.last = r
	return []query.Edge{{From: "a", To: "b", Requests: 2}}, f.err
}

func (f *fakeQuerier) SearchLogs(_ context.Context, q query.LogQuery) ([]query.LogRecord, error) {
	f.last = q
	if q.Query == `"bad` {
		return nil, fmt.Errorf("compile: %w", logsearch.ErrSyntax)
	}
	return []query.LogRecord{{Body: "hello"}}, f.err
}

func (f *fakeQuerier) LogFacet(_ context.Context, q query.LogQuery, key string, limit int) ([]query.FacetValue, error) {
	f.last = []any{q, key, limit}
	return []query.FacetValue{{Value: "checkout", Count: 3}}, f.err
}

func (f *fakeQuerier) LogHistogram(_ context.Context, q query.LogQuery, step time.Duration) ([]query.HistogramBucket, error) {
	f.last = []any{q, step}
	return []query.HistogramBucket{}, f.err
}

func (f *fakeQuerier) SearchTraces(_ context.Context, q query.TraceQuery) ([]query.TraceSummary, error) {
	f.last = q
	return []query.TraceSummary{}, f.err
}

func (f *fakeQuerier) Trace(_ context.Context, id string, r query.TimeRange) ([]query.Span, error) {
	f.last = []any{id, r}
	return []query.Span{{SpanID: "s1"}}, f.err
}

func (f *fakeQuerier) Metrics(_ context.Context, r query.TimeRange) ([]query.MetricInfo, error) {
	f.last = r
	return []query.MetricInfo{}, f.err
}

func (f *fakeQuerier) QueryMetric(_ context.Context, q query.MetricQuery) ([]query.Series, error) {
	f.last = q
	return []query.Series{}, f.err
}

func get(t *testing.T, q *fakeQuerier, path string) (int, map[string]any) {
	t.Helper()
	h := api.NewHandler(q, api.Options{Now: func() time.Time { return now }})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET %s: invalid JSON %q: %v", path, rec.Body.String(), err)
	}
	return rec.Code, body
}

var lastHour = query.TimeRange{From: now.Add(-time.Hour), To: now}

func TestServices(t *testing.T) {
	q := &fakeQuerier{}
	code, body := get(t, q, "/api/v1/services")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if !reflect.DeepEqual(q.last, lastHour) {
		t.Errorf("default range = %+v, want the past hour", q.last)
	}
	data := body["data"].([]any)
	if data[0].(map[string]any)["name"] != "api" {
		t.Errorf("body = %v", body)
	}
}

func TestTimeRangeParameters(t *testing.T) {
	q := &fakeQuerier{}
	get(t, q, "/api/v1/services?from=now-7d&to=now-1d")
	want := query.TimeRange{From: now.Add(-7 * 24 * time.Hour), To: now.Add(-24 * time.Hour)}
	if !reflect.DeepEqual(q.last, want) {
		t.Errorf("range = %+v, want %+v", q.last, want)
	}

	get(t, q, "/api/v1/services?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z")
	want = query.TimeRange{
		From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	}
	if !reflect.DeepEqual(q.last, want) {
		t.Errorf("absolute range = %+v, want %+v", q.last, want)
	}

	for _, bad := range []string{"from=yesterday", "from=now&to=now-1h", "to=now-5y"} {
		if code, body := get(t, q, "/api/v1/services?"+bad); code != http.StatusBadRequest || body["error"] == "" {
			t.Errorf("%s: status %d body %v, want 400 with error", bad, code, body)
		}
	}
}

func TestLogs(t *testing.T) {
	q := &fakeQuerier{}
	code, _ := get(t, q, "/api/v1/logs?q=level:error&limit=20")
	want := query.LogQuery{TimeRange: lastHour, Query: "level:error", Limit: 20}
	if code != http.StatusOK || !reflect.DeepEqual(q.last, want) {
		t.Errorf("status %d, query %+v, want %+v", code, q.last, want)
	}

	if code, body := get(t, q, `/api/v1/logs?q=%22bad`); code != http.StatusBadRequest || body["error"] == "" {
		t.Errorf("syntax error: status %d body %v, want 400", code, body)
	}
}

func TestLogHistogram(t *testing.T) {
	q := &fakeQuerier{}
	code, _ := get(t, q, "/api/v1/logs/histogram?q=api&step=5m")
	want := []any{query.LogQuery{TimeRange: lastHour, Query: "api"}, 5 * time.Minute}
	if code != http.StatusOK || !reflect.DeepEqual(q.last, want) {
		t.Errorf("status %d, call %+v, want %+v", code, q.last, want)
	}
}

func TestTraces(t *testing.T) {
	q := &fakeQuerier{}
	code, _ := get(t, q, "/api/v1/traces?service=api&errors=true&min_duration_ms=250&limit=10")
	want := query.TraceQuery{TimeRange: lastHour, Service: "api", ErrorsOnly: true,
		MinDuration: 250 * time.Millisecond, Limit: 10}
	if code != http.StatusOK || !reflect.DeepEqual(q.last, want) {
		t.Errorf("status %d, query %+v, want %+v", code, q.last, want)
	}
}

func TestTraceByID(t *testing.T) {
	q := &fakeQuerier{}
	code, body := get(t, q, "/api/v1/traces/abc123")
	if code != http.StatusOK || !reflect.DeepEqual(q.last, []any{"abc123", query.TimeRange{}}) {
		t.Errorf("status %d, call %+v; a trace without range scans everything", code, q.last)
	}
	if body["data"].([]any)[0].(map[string]any)["span_id"] != "s1" {
		t.Errorf("body = %v", body)
	}
}

func TestTraceNotFound(t *testing.T) {
	q := &emptyTraceQuerier{}
	h := api.NewHandler(q, api.Options{Now: func() time.Time { return now }})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/traces/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

type emptyTraceQuerier struct{ fakeQuerier }

func (*emptyTraceQuerier) Trace(context.Context, string, query.TimeRange) ([]query.Span, error) {
	return []query.Span{}, nil
}

func TestMetricQuery(t *testing.T) {
	q := &fakeQuerier{}
	code, _ := get(t, q, "/api/v1/metrics/query?metric=http.server.request.duration&agg=p95"+
		"&group_by=http.route,service.name&filter=service.name:api&filter=http.route:/pay&step=1m")
	want := query.MetricQuery{
		TimeRange: lastHour, Metric: "http.server.request.duration", Agg: "p95",
		GroupBy: []string{"http.route", "service.name"},
		Filters: map[string]string{"service.name": "api", "http.route": "/pay"},
		Step:    time.Minute,
	}
	if code != http.StatusOK || !reflect.DeepEqual(q.last, want) {
		t.Errorf("status %d, query %+v, want %+v", code, q.last, want)
	}

	if code, _ := get(t, q, "/api/v1/metrics/query"); code != http.StatusBadRequest {
		t.Errorf("missing metric: status %d, want 400", code)
	}
}

func TestInternalErrorsAreReported(t *testing.T) {
	code, body := get(t, &fakeQuerier{err: fmt.Errorf("disk on fire")}, "/api/v1/metrics")
	if code != http.StatusInternalServerError || body["error"] == "" {
		t.Errorf("status %d body %v, want 500 with error", code, body)
	}
}

func TestServiceDetail(t *testing.T) {
	q := &fakeQuerier{}
	code, body := get(t, q, "/api/v1/services/checkout?step=1m")
	if code != http.StatusOK || !reflect.DeepEqual(q.last, []any{"checkout", lastHour, time.Minute}) {
		t.Errorf("status %d, call %+v", code, q.last)
	}
	if body["data"].(map[string]any)["summary"].(map[string]any)["name"] != "checkout" {
		t.Errorf("body = %v", body)
	}
}

func TestServiceDetailNotFound(t *testing.T) {
	q := &unknownServiceQuerier{}
	h := api.NewHandler(q, api.Options{Now: func() time.Time { return now }})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/services/ghost", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

type unknownServiceQuerier struct{ fakeQuerier }

func (*unknownServiceQuerier) ServiceDetail(_ context.Context, name string, _ query.TimeRange, _ time.Duration) (query.ServiceDetail, error) {
	return query.ServiceDetail{Summary: query.ServiceSummary{Name: name}}, nil
}

func TestServiceMap(t *testing.T) {
	q := &fakeQuerier{}
	code, body := get(t, q, "/api/v1/service-map")
	if code != http.StatusOK || !reflect.DeepEqual(q.last, lastHour) {
		t.Errorf("status %d, call %+v", code, q.last)
	}
	if body["data"].([]any)[0].(map[string]any)["from"] != "a" {
		t.Errorf("body = %v", body)
	}
}

func TestLogFacet(t *testing.T) {
	q := &fakeQuerier{}
	code, body := get(t, q, "/api/v1/logs/facets?key=service.name&q=level:error")
	want := []any{query.LogQuery{TimeRange: lastHour, Query: "level:error"}, "service.name", 20}
	if code != http.StatusOK || !reflect.DeepEqual(q.last, want) {
		t.Errorf("status %d, call %+v, want %+v", code, q.last, want)
	}
	if body["data"].([]any)[0].(map[string]any)["value"] != "checkout" {
		t.Errorf("body = %v", body)
	}
	if code, _ := get(t, q, "/api/v1/logs/facets"); code != http.StatusBadRequest {
		t.Errorf("missing key: %d, want 400", code)
	}
}
