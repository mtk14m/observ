package query

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "github.com/duckdb/duckdb-go/v2" // registers the "duckdb" driver
	"golang.org/x/sync/errgroup"

	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

const (
	defaultLogLimit   = 100
	maxLogLimit       = 1000
	defaultTraceLimit = 50
	maxTraceLimit     = 500
	// counterLookback widens metric scans so the first bucket of a
	// cumulative series has a baseline.
	counterLookback = 15 * time.Minute
)

// FileSource lists stored objects and resolves them to local files, for
// example the filesystem store, or a cache in front of S3.
type FileSource interface {
	List(ctx context.Context, prefix string) ([]objstore.ObjectInfo, error)
	Local(ctx context.Context, key string) (string, error)
}

// fetchConcurrency bounds parallel downloads when resolving files.
const fetchConcurrency = 8

// TimeRange is a half-open interval [From, To). A zero range means "all".
type TimeRange struct {
	From, To time.Time
}

func (r TimeRange) bounds() (from, to int64) {
	return r.From.UnixNano(), r.To.UnixNano()
}

// HotSource exposes Parquet snapshots of data not yet in the store.
type HotSource interface {
	HotFiles(dir string) ([]string, error)
}

// Option configures an Engine.
type Option func(*Engine)

// WithHot makes unflushed data queryable.
func WithHot(h HotSource) Option { return func(e *Engine) { e.hot = h } }

// Engine runs queries over the Parquet files of a FileSource.
type Engine struct {
	db  *sql.DB
	src FileSource
	hot HotSource
}

// New opens an in-memory DuckDB instance to query src.
func New(src FileSource, opts ...Option) (*Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("query: open duckdb: %w", err)
	}
	e := &Engine{db: db, src: src}
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

// Close releases DuckDB.
func (e *Engine) Close() error { return e.db.Close() }

// files returns the local paths of the files of a signal that may hold data
// in r, using the date/hour partitions of their keys.
func (e *Engine) files(ctx context.Context, dir string, r TimeRange) ([]string, error) {
	infos, err := e.src.List(ctx, layout.Prefix(dir))
	if err != nil {
		return nil, fmt.Errorf("query: list %s: %w", dir, err)
	}
	var keys []string
	for _, info := range infos {
		k, ok := layout.ParseKey(info.Key)
		if !ok {
			continue
		}
		if !r.From.IsZero() && (!k.Hour.Add(time.Hour).After(r.From) || !k.Hour.Before(r.To)) {
			continue
		}
		keys = append(keys, info.Key)
	}
	paths := make([]string, len(keys))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(fetchConcurrency)
	for i, key := range keys {
		g.Go(func() error {
			p, err := e.src.Local(gctx, key)
			if errors.Is(err, objstore.ErrNotFound) {
				return nil // removed by compaction since the listing
			}
			paths[i] = p
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("query: fetch %s: %w", dir, err)
	}
	paths = slices.DeleteFunc(paths, func(p string) bool { return p == "" })
	if e.hot != nil {
		hot, err := e.hot.HotFiles(dir)
		if err != nil {
			return nil, fmt.Errorf("query: hot %s: %w", dir, err)
		}
		paths = append(paths, hot...)
	}
	return paths, nil
}

// source builds a DuckDB table expression over paths.
func source(paths []string) string {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = "'" + strings.ReplaceAll(p, "'", "''") + "'"
	}
	return "read_parquet([" + strings.Join(quoted, ", ") + "], union_by_name = true)"
}

// ServiceSummary describes the entry-point spans of a service.
type ServiceSummary struct {
	Name          string  `json:"name"`
	Requests      int64   `json:"requests"`
	Errors        int64   `json:"errors"`
	ErrorRate     float64 `json:"error_rate"`
	RatePerSecond float64 `json:"rate_per_second"`
	P50Ms         float64 `json:"p50_ms"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
}

// Services returns request, error and latency statistics per service,
// computed from server, consumer and root spans.
func (e *Engine) Services(ctx context.Context, r TimeRange) ([]ServiceSummary, error) {
	paths, err := e.files(ctx, layout.Spans, r)
	if err != nil || len(paths) == 0 {
		return []ServiceSummary{}, err
	}
	from, to := r.bounds()
	rows, err := e.db.QueryContext(ctx, `
		SELECT service_name, count(*) AS requests,
		       count(*) FILTER (WHERE status_code = 'Error'),
		       quantile_cont(duration_nano, 0.5) / 1e6,
		       quantile_cont(duration_nano, 0.95) / 1e6,
		       quantile_cont(duration_nano, 0.99) / 1e6
		FROM `+source(paths)+`
		WHERE start_time_unix_nano >= ? AND start_time_unix_nano < ?
		  AND (kind IN ('Server', 'Consumer') OR parent_span_id = '')
		GROUP BY service_name
		ORDER BY requests DESC, service_name`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query: services: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seconds := r.To.Sub(r.From).Seconds()
	out := []ServiceSummary{}
	for rows.Next() {
		var s ServiceSummary
		if err := rows.Scan(&s.Name, &s.Requests, &s.Errors, &s.P50Ms, &s.P95Ms, &s.P99Ms); err != nil {
			return nil, fmt.Errorf("query: services: %w", err)
		}
		if s.Requests > 0 {
			s.ErrorRate = float64(s.Errors) / float64(s.Requests)
		}
		if seconds > 0 {
			s.RatePerSecond = float64(s.Requests) / seconds
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LogQuery selects log records.
type LogQuery struct {
	TimeRange
	// Query uses the logsearch syntax.
	Query string
	Limit int
}

// LogRecord is a log record as returned by the API.
type LogRecord struct {
	Time               int64             `json:"time"`
	Service            string            `json:"service"`
	Severity           string            `json:"severity"`
	SeverityNumber     int32             `json:"severity_number"`
	Body               string            `json:"body"`
	TraceID            string            `json:"trace_id"`
	SpanID             string            `json:"span_id"`
	Attributes         map[string]string `json:"attributes"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
}

// SearchLogs returns matching log records, newest first.
func (e *Engine) SearchLogs(ctx context.Context, q LogQuery) ([]LogRecord, error) {
	pred, err := logsearch.Compile(q.Query)
	if err != nil {
		return nil, err
	}
	paths, err := e.files(ctx, layout.Logs, q.TimeRange)
	if err != nil || len(paths) == 0 {
		return []LogRecord{}, err
	}
	from, to := q.bounds()
	args := append([]any{from, to}, pred.Args...)
	args = append(args, clamp(q.Limit, defaultLogLimit, maxLogLimit))
	rows, err := e.db.QueryContext(ctx, `
		SELECT time_unix_nano, service_name, severity_text, severity_number, body, trace_id, span_id,
		       to_json(attributes)::VARCHAR, to_json(resource_attributes)::VARCHAR
		FROM `+source(paths)+`
		WHERE time_unix_nano >= ? AND time_unix_nano < ? AND (`+pred.Where+`)
		ORDER BY time_unix_nano DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query: logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []LogRecord{}
	for rows.Next() {
		var l LogRecord
		var attrs, res string
		if err := rows.Scan(&l.Time, &l.Service, &l.Severity, &l.SeverityNumber, &l.Body,
			&l.TraceID, &l.SpanID, &attrs, &res); err != nil {
			return nil, fmt.Errorf("query: logs: %w", err)
		}
		l.Attributes, l.ResourceAttributes = jsonMap(attrs), jsonMap(res)
		out = append(out, l)
	}
	return out, rows.Err()
}

// HistogramBucket counts log records per severity in [T, T+step).
type HistogramBucket struct {
	T      int64            `json:"t"`
	Counts map[string]int64 `json:"counts"`
}

// LogHistogram counts matching records per severity and time bucket.
func (e *Engine) LogHistogram(ctx context.Context, q LogQuery, step time.Duration) ([]HistogramBucket, error) {
	pred, err := logsearch.Compile(q.Query)
	if err != nil {
		return nil, err
	}
	paths, err := e.files(ctx, layout.Logs, q.TimeRange)
	if err != nil || len(paths) == 0 {
		return []HistogramBucket{}, err
	}
	if step <= 0 {
		step = NiceStep(q.To.Sub(q.From), 60)
	}
	from, to := q.bounds()
	args := append([]any{from, step.Nanoseconds(), from, to}, pred.Args...)
	rows, err := e.db.QueryContext(ctx, `
		SELECT (time_unix_nano - ?) // ? AS bucket, coalesce(nullif(upper(severity_text), ''), 'UNSET'), count(*)
		FROM `+source(paths)+`
		WHERE time_unix_nano >= ? AND time_unix_nano < ? AND (`+pred.Where+`)
		GROUP BY ALL
		ORDER BY bucket`, args...)
	if err != nil {
		return nil, fmt.Errorf("query: log histogram: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []HistogramBucket{}
	for rows.Next() {
		var bucket, count int64
		var severity string
		if err := rows.Scan(&bucket, &severity, &count); err != nil {
			return nil, fmt.Errorf("query: log histogram: %w", err)
		}
		t := from + bucket*step.Nanoseconds()
		if n := len(out); n == 0 || out[n-1].T != t {
			out = append(out, HistogramBucket{T: t, Counts: map[string]int64{}})
		}
		out[len(out)-1].Counts[severity] = count
	}
	return out, rows.Err()
}

// TraceQuery selects traces.
type TraceQuery struct {
	TimeRange
	// Query keeps traces with at least one span matching it (span search syntax).
	Query       string
	Service     string
	ErrorsOnly  bool
	MinDuration time.Duration
	Limit       int
}

// TraceSummary describes a trace in search results.
type TraceSummary struct {
	TraceID      string   `json:"trace_id"`
	RootService  string   `json:"root_service"`
	RootName     string   `json:"root_name"`
	Start        int64    `json:"start"`
	DurationNano int64    `json:"duration_nano"`
	SpanCount    int64    `json:"span_count"`
	ErrorCount   int64    `json:"error_count"`
	Services     []string `json:"services"`
}

// SearchTraces returns matching traces, most recent first.
func (e *Engine) SearchTraces(ctx context.Context, q TraceQuery) ([]TraceSummary, error) {
	pred, err := logsearch.CompileSpans(q.Query)
	if err != nil {
		return nil, err
	}
	paths, err := e.files(ctx, layout.Spans, q.TimeRange)
	if err != nil || len(paths) == 0 {
		return []TraceSummary{}, err
	}
	from, to := q.bounds()
	args := []any{from, to}
	having := "TRUE"
	if q.Service != "" {
		having += " AND bool_or(service_name = ?)"
		args = append(args, q.Service)
	}
	if q.ErrorsOnly {
		having += " AND count(*) FILTER (WHERE status_code = 'Error') > 0"
	}
	if q.MinDuration > 0 {
		having += " AND max(end_time_unix_nano) - min(start_time_unix_nano) >= ?"
		args = append(args, q.MinDuration.Nanoseconds())
	}
	if q.Query != "" {
		having += " AND bool_or(" + pred.Where + ")"
		args = append(args, pred.Args...)
	}
	args = append(args, clamp(q.Limit, defaultTraceLimit, maxTraceLimit))

	rows, err := e.db.QueryContext(ctx, `
		SELECT trace_id,
		       coalesce(any_value(service_name) FILTER (WHERE parent_span_id = ''), arg_min(service_name, start_time_unix_nano)),
		       coalesce(any_value(name) FILTER (WHERE parent_span_id = ''), arg_min(name, start_time_unix_nano)),
		       min(start_time_unix_nano) AS start,
		       max(end_time_unix_nano) - min(start_time_unix_nano),
		       count(*),
		       count(*) FILTER (WHERE status_code = 'Error'),
		       to_json(list_sort(list_distinct(list(service_name))))::VARCHAR
		FROM `+source(paths)+`
		WHERE start_time_unix_nano >= ? AND start_time_unix_nano < ?
		GROUP BY trace_id
		HAVING `+having+`
		ORDER BY start DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query: traces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []TraceSummary{}
	for rows.Next() {
		var s TraceSummary
		var services string
		if err := rows.Scan(&s.TraceID, &s.RootService, &s.RootName, &s.Start, &s.DurationNano,
			&s.SpanCount, &s.ErrorCount, &services); err != nil {
			return nil, fmt.Errorf("query: traces: %w", err)
		}
		_ = json.Unmarshal([]byte(services), &s.Services)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Span is a span as returned by the API.
type Span struct {
	TraceID            string            `json:"trace_id"`
	SpanID             string            `json:"span_id"`
	ParentSpanID       string            `json:"parent_span_id"`
	Name               string            `json:"name"`
	Kind               string            `json:"kind"`
	Start              int64             `json:"start"`
	End                int64             `json:"end"`
	DurationNano       int64             `json:"duration_nano"`
	StatusCode         string            `json:"status_code"`
	StatusMessage      string            `json:"status_message"`
	Service            string            `json:"service"`
	ScopeName          string            `json:"scope_name"`
	Attributes         map[string]string `json:"attributes"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
	Events             []SpanEvent       `json:"events"`
}

// SpanEvent is an event of a span.
type SpanEvent struct {
	Time       int64             `json:"time_unix_nano"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

// Trace returns every span of a trace, ordered by start time. A zero range
// scans every file.
func (e *Engine) Trace(ctx context.Context, traceID string, r TimeRange) ([]Span, error) {
	paths, err := e.files(ctx, layout.Spans, r)
	if err != nil || len(paths) == 0 {
		return []Span{}, err
	}
	rows, err := e.db.QueryContext(ctx, `
		SELECT trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano,
		       duration_nano, status_code, status_message, service_name, scope_name,
		       to_json(attributes)::VARCHAR, to_json(resource_attributes)::VARCHAR, to_json(events)::VARCHAR
		FROM `+source(paths)+`
		WHERE trace_id = ?
		ORDER BY start_time_unix_nano, span_id`, traceID)
	if err != nil {
		return nil, fmt.Errorf("query: trace: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []Span{}
	for rows.Next() {
		var s Span
		var attrs, res, events string
		if err := rows.Scan(&s.TraceID, &s.SpanID, &s.ParentSpanID, &s.Name, &s.Kind, &s.Start, &s.End,
			&s.DurationNano, &s.StatusCode, &s.StatusMessage, &s.Service, &s.ScopeName,
			&attrs, &res, &events); err != nil {
			return nil, fmt.Errorf("query: trace: %w", err)
		}
		s.Attributes, s.ResourceAttributes = jsonMap(attrs), jsonMap(res)
		s.Events = []SpanEvent{}
		_ = json.Unmarshal([]byte(events), &s.Events)
		out = append(out, s)
	}
	return out, rows.Err()
}

// MetricInfo describes a metric available for querying.
type MetricInfo struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Unit          string   `json:"unit"`
	Series        int64    `json:"series"`
	AttributeKeys []string `json:"attribute_keys"`
	// Monotonic is true for counters, which are queried as rates.
	Monotonic bool `json:"monotonic"`
}

// Metrics lists the metrics with data in r.
func (e *Engine) Metrics(ctx context.Context, r TimeRange) ([]MetricInfo, error) {
	paths, err := e.files(ctx, layout.MetricPoints, r)
	if err != nil || len(paths) == 0 {
		return []MetricInfo{}, err
	}
	from, to := r.bounds()
	rows, err := e.db.QueryContext(ctx, `
		SELECT metric_name, any_value(type), any_value(unit), count(DISTINCT series_id),
		       to_json(list_sort(list_distinct(flatten(list(map_keys(attributes))))))::VARCHAR,
		       bool_or(is_monotonic)
		FROM `+source(paths)+`
		WHERE time_unix_nano >= ? AND time_unix_nano < ?
		GROUP BY metric_name
		ORDER BY metric_name`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query: metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []MetricInfo{}
	for rows.Next() {
		var m MetricInfo
		var keys string
		if err := rows.Scan(&m.Name, &m.Type, &m.Unit, &m.Series, &keys, &m.Monotonic); err != nil {
			return nil, fmt.Errorf("query: metrics: %w", err)
		}
		_ = json.Unmarshal([]byte(keys), &m.AttributeKeys)
		m.AttributeKeys = append(m.AttributeKeys, "service.name")
		out = append(out, m)
	}
	return out, rows.Err()
}

// MetricQuery selects and aggregates one metric.
type MetricQuery struct {
	TimeRange
	Metric string
	// Agg is avg, sum, min, max or count for gauges and counters; count,
	// avg or a percentile (p50, p95, p99…) for histograms. Empty picks a
	// sensible default for the metric type.
	Agg string
	// GroupBy lists attribute keys (record or resource) to split series by.
	GroupBy []string
	// Filters keeps points whose attribute equals the value.
	Filters map[string]string
	// Step defaults to about 120 points over the range.
	Step time.Duration
}

// QueryMetric returns one series per group-by combination.
func (e *Engine) QueryMetric(ctx context.Context, q MetricQuery) ([]Series, error) {
	if q.Step <= 0 {
		q.Step = NiceStep(q.To.Sub(q.From), 120)
	}
	scan := TimeRange{From: q.From.Add(-counterLookback), To: q.To}
	paths, err := e.files(ctx, layout.MetricPoints, scan)
	if err != nil || len(paths) == 0 {
		return []Series{}, err
	}
	src := source(paths)

	var kind metricKind
	var typ, temporality sql.NullString
	var monotonic sql.NullBool
	if err := e.db.QueryRowContext(ctx, `
		SELECT any_value(type), any_value(temporality), bool_or(is_monotonic)
		FROM `+src+` WHERE metric_name = ?`, q.Metric).Scan(&typ, &temporality, &monotonic); err != nil {
		return nil, fmt.Errorf("query: metric kind: %w", err)
	}
	if !typ.Valid {
		return []Series{}, nil
	}
	kind = metricKind{Type: typ.String, Temporality: temporality.String, Monotonic: monotonic.Bool}

	cols := []string{"series_id", "start_time_unix_nano", "time_unix_nano", "value", "count", "sum",
		"to_json(explicit_bounds)::VARCHAR", "to_json(bucket_counts)::VARCHAR"}
	var args []any
	for _, k := range q.GroupBy {
		cols = append(cols, "coalesce(attributes[?], resource_attributes[?], '')")
		args = append(args, k, k)
	}
	from, to := scan.bounds()
	args = append(args, q.Metric, from, to)
	where := "metric_name = ? AND time_unix_nano >= ? AND time_unix_nano < ?"
	for k, v := range q.Filters {
		where += " AND coalesce(attributes[?], resource_attributes[?]) = ?"
		args = append(args, k, k, v)
	}

	rows, err := e.db.QueryContext(ctx, `SELECT `+strings.Join(cols, ", ")+` FROM `+src+
		` WHERE `+where+` ORDER BY series_id, time_unix_nano`, args...)
	if err != nil {
		return nil, fmt.Errorf("query: metric points: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var points []rawPoint
	for rows.Next() {
		var p rawPoint
		var bounds, buckets string
		groupVals := make([]string, len(q.GroupBy))
		dest := []any{&p.SeriesID, &p.Start, &p.Time, &p.Value, &p.Count, &p.Sum, &bounds, &buckets}
		for i := range groupVals {
			dest = append(dest, &groupVals[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("query: metric points: %w", err)
		}
		_ = json.Unmarshal([]byte(bounds), &p.Bounds)
		_ = json.Unmarshal([]byte(buckets), &p.Buckets)
		p.Labels = make(map[string]string, len(q.GroupBy))
		for i, k := range q.GroupBy {
			p.Labels[k] = groupVals[i]
		}
		p.Group = labelKey(p.Labels)
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	from, to = q.bounds()
	return computeSeries(kind, points, from, to, q.Step.Nanoseconds(), q.Agg), nil
}

// NiceStep returns a round step giving at most about n buckets over d.
func NiceStep(d time.Duration, n int) time.Duration {
	steps := []time.Duration{
		10 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute,
		10 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour,
		6 * time.Hour, 12 * time.Hour, 24 * time.Hour,
	}
	for _, s := range steps {
		if d/s <= time.Duration(n) {
			return s
		}
	}
	return steps[len(steps)-1]
}

func clamp(v, def, maxV int) int {
	if v <= 0 {
		return def
	}
	return min(v, maxV)
}

func jsonMap(s string) map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

// GroupCount is the number of matching records for one group.
type GroupCount struct {
	Labels map[string]string `json:"labels"`
	Count  int64             `json:"count"`
}

// CountLogs counts matching log records, per value of the groupBy
// attributes (record or resource; "level" is the severity). Without groupBy it always returns one
// count, possibly zero.
func (e *Engine) CountLogs(ctx context.Context, q LogQuery, groupBy []string) ([]GroupCount, error) {
	pred, err := logsearch.Compile(q.Query)
	if err != nil {
		return nil, err
	}
	empty := []GroupCount{}
	if len(groupBy) == 0 {
		empty = []GroupCount{{Labels: map[string]string{}}}
	}
	paths, err := e.files(ctx, layout.Logs, q.TimeRange)
	if err != nil || len(paths) == 0 {
		return empty, err
	}
	cols := make([]string, len(groupBy))
	var args []any
	for i, k := range groupBy {
		if k == "level" {
			cols[i] = "coalesce(nullif(upper(severity_text), ''), 'UNSET')"
			continue
		}
		cols[i] = "coalesce(attributes[?], resource_attributes[?], '')"
		args = append(args, k, k)
	}
	from, to := q.bounds()
	args = append(args, from, to)
	args = append(args, pred.Args...)
	sel := strings.Join(append(slices.Clone(cols), "count(*)"), ", ")
	group := ""
	if len(cols) > 0 {
		group = " GROUP BY ALL ORDER BY ALL"
	}
	rows, err := e.db.QueryContext(ctx, `SELECT `+sel+` FROM `+source(paths)+`
		WHERE time_unix_nano >= ? AND time_unix_nano < ? AND (`+pred.Where+`)`+group, args...)
	if err != nil {
		return nil, fmt.Errorf("query: count logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []GroupCount{}
	for rows.Next() {
		vals := make([]string, len(groupBy))
		var c GroupCount
		dest := make([]any, 0, len(groupBy)+1)
		for i := range vals {
			dest = append(dest, &vals[i])
		}
		dest = append(dest, &c.Count)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("query: count logs: %w", err)
		}
		c.Labels = make(map[string]string, len(groupBy))
		for i, k := range groupBy {
			c.Labels[k] = vals[i]
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return empty, rows.Err()
	}
	return out, rows.Err()
}

// FacetValue is one value of a log attribute and its number of records.
type FacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// LogFacet returns the most frequent values of an attribute (or "level")
// among matching log records, most frequent first.
func (e *Engine) LogFacet(ctx context.Context, q LogQuery, key string, limit int) ([]FacetValue, error) {
	counts, err := e.CountLogs(ctx, q, []string{key})
	if err != nil {
		return nil, err
	}
	out := make([]FacetValue, 0, len(counts))
	for _, c := range counts {
		if c.Count > 0 {
			out = append(out, FacetValue{Value: c.Labels[key], Count: c.Count})
		}
	}
	slices.SortFunc(out, func(a, b FacetValue) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), strings.Compare(a.Value, b.Value))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
