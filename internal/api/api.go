// Package api serves the obsrv HTTP JSON API under /api/v1.
//
// Every endpoint accepts from and to parameters using the same time
// expressions as the UI: "now", "now-15m", "now-7d" or an RFC 3339 date.
// Responses are {"data": …} on success and {"error": "…"} on failure.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mtk14n/obsrv/internal/query"
	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

// Querier answers the API's questions. *query.Engine implements it.
type Querier interface {
	Services(ctx context.Context, r query.TimeRange) ([]query.ServiceSummary, error)
	ServiceDetail(ctx context.Context, name string, r query.TimeRange, step time.Duration) (query.ServiceDetail, error)
	ServiceMap(ctx context.Context, r query.TimeRange) ([]query.Edge, error)
	SearchLogs(ctx context.Context, q query.LogQuery) ([]query.LogRecord, error)
	LogHistogram(ctx context.Context, q query.LogQuery, step time.Duration) ([]query.HistogramBucket, error)
	LogFacet(ctx context.Context, q query.LogQuery, key string, limit int) ([]query.FacetValue, error)
	SearchTraces(ctx context.Context, q query.TraceQuery) ([]query.TraceSummary, error)
	Trace(ctx context.Context, traceID string, r query.TimeRange) ([]query.Span, error)
	Metrics(ctx context.Context, r query.TimeRange) ([]query.MetricInfo, error)
	QueryMetric(ctx context.Context, q query.MetricQuery) ([]query.Series, error)
}

var _ Querier = (*query.Engine)(nil)

// Options configures the handler.
type Options struct {
	Now func() time.Time
	// Alerts enables the alerting endpoints when set.
	Alerts *Alerts
}

// errBadRequest marks client errors (400).
var errBadRequest = errors.New("bad request")

// NewHandler returns the API handler.
func NewHandler(q Querier, opts Options) http.Handler {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	h := &handler{q: q, now: opts.Now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/services", h.services)
	mux.HandleFunc("GET /api/v1/services/{name}", h.serviceDetail)
	mux.HandleFunc("GET /api/v1/service-map", h.serviceMap)
	mux.HandleFunc("GET /api/v1/logs", h.logs)
	mux.HandleFunc("GET /api/v1/logs/histogram", h.logHistogram)
	mux.HandleFunc("GET /api/v1/logs/facets", h.logFacet)
	mux.HandleFunc("GET /api/v1/traces", h.traces)
	mux.HandleFunc("GET /api/v1/traces/{id}", h.trace)
	mux.HandleFunc("GET /api/v1/metrics", h.metrics)
	mux.HandleFunc("GET /api/v1/metrics/query", h.metricQuery)
	if opts.Alerts != nil {
		opts.Alerts.register(mux)
	}
	return mux
}

type handler struct {
	q   Querier
	now func() time.Time
}

func (h *handler) services(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.Services(r.Context(), tr)
	write(w, data, err)
}

func (h *handler) serviceDetail(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	step, err := durationParam(r, "step")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.ServiceDetail(r.Context(), r.PathValue("name"), tr, step)
	if err == nil && data.Summary.Requests == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no requests for this service in the time range"})
		return
	}
	write(w, data, err)
}

func (h *handler) serviceMap(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.ServiceMap(r.Context(), tr)
	write(w, data, err)
}

func (h *handler) logQuery(r *http.Request) (query.LogQuery, error) {
	tr, err := h.timeRange(r)
	if err != nil {
		return query.LogQuery{}, err
	}
	limit, err := intParam(r, "limit")
	return query.LogQuery{TimeRange: tr, Query: r.URL.Query().Get("q"), Limit: limit}, err
}

func (h *handler) logs(w http.ResponseWriter, r *http.Request) {
	q, err := h.logQuery(r)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.SearchLogs(r.Context(), q)
	write(w, data, err)
}

func (h *handler) logHistogram(w http.ResponseWriter, r *http.Request) {
	q, err := h.logQuery(r)
	if err != nil {
		writeError(w, err)
		return
	}
	step, err := durationParam(r, "step")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.LogHistogram(r.Context(), q, step)
	write(w, data, err)
}

func (h *handler) logFacet(w http.ResponseWriter, r *http.Request) {
	q, err := h.logQuery(r)
	if err != nil {
		writeError(w, err)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, fmt.Errorf("%w: key is required", errBadRequest))
		return
	}
	data, err := h.q.LogFacet(r.Context(), query.LogQuery{TimeRange: q.TimeRange, Query: q.Query}, key, 20)
	write(w, data, err)
}

func (h *handler) traces(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := intParam(r, "limit")
	if err != nil {
		writeError(w, err)
		return
	}
	minMs, err := intParam(r, "min_duration_ms")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.SearchTraces(r.Context(), query.TraceQuery{
		TimeRange:   tr,
		Service:     r.URL.Query().Get("service"),
		ErrorsOnly:  r.URL.Query().Get("errors") == "true",
		MinDuration: time.Duration(minMs) * time.Millisecond,
		Limit:       limit,
	})
	write(w, data, err)
}

func (h *handler) trace(w http.ResponseWriter, r *http.Request) {
	// A trace link must work whatever its age: without an explicit range,
	// every file is scanned.
	var tr query.TimeRange
	if r.URL.Query().Has("from") {
		var err error
		if tr, err = h.timeRange(r); err != nil {
			writeError(w, err)
			return
		}
	}
	data, err := h.q.Trace(r.Context(), r.PathValue("id"), tr)
	if err == nil && len(data) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
		return
	}
	write(w, data, err)
}

func (h *handler) metrics(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.Metrics(r.Context(), tr)
	write(w, data, err)
}

func (h *handler) metricQuery(w http.ResponseWriter, r *http.Request) {
	tr, err := h.timeRange(r)
	if err != nil {
		writeError(w, err)
		return
	}
	params := r.URL.Query()
	q := query.MetricQuery{TimeRange: tr, Metric: params.Get("metric"), Agg: params.Get("agg")}
	if q.Metric == "" {
		writeError(w, fmt.Errorf("%w: metric is required", errBadRequest))
		return
	}
	if g := params.Get("group_by"); g != "" {
		q.GroupBy = strings.Split(g, ",")
	}
	for _, f := range params["filter"] {
		k, v, ok := strings.Cut(f, ":")
		if !ok || k == "" {
			writeError(w, fmt.Errorf("%w: filter must be key:value, got %q", errBadRequest, f))
			return
		}
		if q.Filters == nil {
			q.Filters = map[string]string{}
		}
		q.Filters[k] = v
	}
	if q.Step, err = durationParam(r, "step"); err != nil {
		writeError(w, err)
		return
	}
	data, err := h.q.QueryMetric(r.Context(), q)
	write(w, data, err)
}

// timeRange parses from/to, defaulting to the past hour.
func (h *handler) timeRange(r *http.Request) (query.TimeRange, error) {
	now := h.now()
	params := r.URL.Query()
	fromExpr, toExpr := params.Get("from"), params.Get("to")
	if fromExpr == "" {
		fromExpr = "now-1h"
	}
	if toExpr == "" {
		toExpr = "now"
	}
	from, err := ParseTime(fromExpr, now)
	if err != nil {
		return query.TimeRange{}, err
	}
	to, err := ParseTime(toExpr, now)
	if err != nil {
		return query.TimeRange{}, err
	}
	if !from.Before(to) {
		return query.TimeRange{}, fmt.Errorf("%w: from must be before to", errBadRequest)
	}
	return query.TimeRange{From: from, To: to}, nil
}

var relative = regexp.MustCompile(`^now(?:-(\d+)([smhdw]))?$`)

var units = map[string]time.Duration{
	"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour,
}

// ParseTime parses "now", "now-<n><s|m|h|d|w>" or an RFC 3339 date. It
// mirrors web/src/lib/timeRange.ts.
func ParseTime(expr string, now time.Time) (time.Time, error) {
	if m := relative.FindStringSubmatch(expr); m != nil {
		if m[1] == "" {
			return now, nil
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: invalid time %q", errBadRequest, expr)
		}
		return now.Add(-time.Duration(n) * units[m[2]]), nil
	}
	t, err := time.Parse(time.RFC3339Nano, expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid time %q", errBadRequest, expr)
	}
	return t.UTC(), nil
}

func intParam(r *http.Request, name string) (int, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: %s must be a positive integer", errBadRequest, name)
	}
	return n, nil
}

func durationParam(r *http.Request, name string) (time.Duration, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%w: %s must be a duration such as 30s or 5m", errBadRequest, name)
	}
	return d, nil
}

func write(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, errBadRequest) || errors.Is(err, logsearch.ErrSyntax) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
