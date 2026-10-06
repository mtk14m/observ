package query

import (
	"context"
	"fmt"
	"time"

	"github.com/mtk14n/obsrv/internal/layout"
)

// entrySpans selects the spans that represent requests handled by a
// service: server and consumer spans, plus roots.
const entrySpans = "(kind IN ('Server', 'Consumer') OR parent_span_id = '')"

// TimelinePoint holds the request statistics of one time bucket.
type TimelinePoint struct {
	T        int64   `json:"t"`
	Requests int64   `json:"requests"`
	Errors   int64   `json:"errors"`
	P50Ms    float64 `json:"p50_ms"`
	P95Ms    float64 `json:"p95_ms"`
	P99Ms    float64 `json:"p99_ms"`
}

// Operation summarises one span name (usually a route) of a service.
type Operation struct {
	Name     string  `json:"name"`
	Requests int64   `json:"requests"`
	Errors   int64   `json:"errors"`
	P50Ms    float64 `json:"p50_ms"`
	P95Ms    float64 `json:"p95_ms"`
}

// Dependency is a service called by, or calling, another service.
type Dependency struct {
	Service  string `json:"service"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

// ServiceDetail is everything the service page shows.
type ServiceDetail struct {
	Summary    ServiceSummary  `json:"summary"`
	Step       int64           `json:"step"`
	Timeline   []TimelinePoint `json:"timeline"`
	Operations []Operation     `json:"operations"`
	Calls      []Dependency    `json:"calls"`
	CalledBy   []Dependency    `json:"called_by"`
}

// Edge is a dependency between two services: From calls To.
type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

// ServiceDetail returns the statistics, timeline, operations and
// dependencies of one service. A zero step picks about 60 buckets.
func (e *Engine) ServiceDetail(ctx context.Context, name string, r TimeRange, step time.Duration) (ServiceDetail, error) {
	if step <= 0 {
		step = NiceStep(r.To.Sub(r.From), 60)
	}
	d := ServiceDetail{
		Summary:  ServiceSummary{Name: name},
		Step:     step.Nanoseconds(),
		Timeline: []TimelinePoint{}, Operations: []Operation{}, Calls: []Dependency{}, CalledBy: []Dependency{},
	}
	services, err := e.Services(ctx, r)
	if err != nil {
		return d, err
	}
	for _, s := range services {
		if s.Name == name {
			d.Summary = s
		}
	}
	if d.Summary.Requests == 0 {
		return d, nil
	}
	paths, err := e.files(ctx, layout.Spans, r)
	if err != nil || len(paths) == 0 {
		return d, err
	}
	src := source(paths)
	from, to := r.bounds()

	rows, err := e.db.QueryContext(ctx, `
		SELECT (start_time_unix_nano - ?) // ? AS bucket, count(*),
		       count(*) FILTER (WHERE status_code = 'Error'),
		       quantile_cont(duration_nano, 0.5) / 1e6,
		       quantile_cont(duration_nano, 0.95) / 1e6,
		       quantile_cont(duration_nano, 0.99) / 1e6
		FROM `+src+`
		WHERE service_name = ? AND start_time_unix_nano >= ? AND start_time_unix_nano < ? AND `+entrySpans+`
		GROUP BY bucket ORDER BY bucket`, from, step.Nanoseconds(), name, from, to)
	if err != nil {
		return d, fmt.Errorf("query: service timeline: %w", err)
	}
	for rows.Next() {
		var p TimelinePoint
		var bucket int64
		if err := rows.Scan(&bucket, &p.Requests, &p.Errors, &p.P50Ms, &p.P95Ms, &p.P99Ms); err != nil {
			_ = rows.Close()
			return d, fmt.Errorf("query: service timeline: %w", err)
		}
		p.T = from + bucket*step.Nanoseconds()
		d.Timeline = append(d.Timeline, p)
	}
	_ = rows.Close()

	rows, err = e.db.QueryContext(ctx, `
		SELECT name, count(*) AS requests, count(*) FILTER (WHERE status_code = 'Error'),
		       quantile_cont(duration_nano, 0.5) / 1e6, quantile_cont(duration_nano, 0.95) / 1e6
		FROM `+src+`
		WHERE service_name = ? AND start_time_unix_nano >= ? AND start_time_unix_nano < ? AND `+entrySpans+`
		GROUP BY name ORDER BY requests DESC, name LIMIT 50`, name, from, to)
	if err != nil {
		return d, fmt.Errorf("query: service operations: %w", err)
	}
	for rows.Next() {
		var o Operation
		if err := rows.Scan(&o.Name, &o.Requests, &o.Errors, &o.P50Ms, &o.P95Ms); err != nil {
			_ = rows.Close()
			return d, fmt.Errorf("query: service operations: %w", err)
		}
		d.Operations = append(d.Operations, o)
	}
	_ = rows.Close()

	edges, err := e.edges(ctx, src, from, to)
	if err != nil {
		return d, err
	}
	for _, edge := range edges {
		if edge.From == name {
			d.Calls = append(d.Calls, Dependency{Service: edge.To, Requests: edge.Requests, Errors: edge.Errors})
		}
		if edge.To == name {
			d.CalledBy = append(d.CalledBy, Dependency{Service: edge.From, Requests: edge.Requests, Errors: edge.Errors})
		}
	}
	return d, nil
}

// ServiceMap returns the calls between services, derived from spans whose
// parent belongs to another service.
func (e *Engine) ServiceMap(ctx context.Context, r TimeRange) ([]Edge, error) {
	paths, err := e.files(ctx, layout.Spans, r)
	if err != nil || len(paths) == 0 {
		return []Edge{}, err
	}
	from, to := r.bounds()
	return e.edges(ctx, source(paths), from, to)
}

func (e *Engine) edges(ctx context.Context, src string, from, to int64) ([]Edge, error) {
	rows, err := e.db.QueryContext(ctx, `
		WITH s AS (
			SELECT trace_id, span_id, parent_span_id, service_name, status_code
			FROM `+src+`
			WHERE start_time_unix_nano >= ? AND start_time_unix_nano < ?
		)
		SELECT p.service_name, c.service_name, count(*) AS requests,
		       count(*) FILTER (WHERE c.status_code = 'Error')
		FROM s c JOIN s p ON c.trace_id = p.trace_id AND c.parent_span_id = p.span_id
		WHERE c.service_name <> p.service_name
		GROUP BY ALL
		ORDER BY requests DESC, 1, 2`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query: service map: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Edge{}
	for rows.Next() {
		var edge Edge
		if err := rows.Scan(&edge.From, &edge.To, &edge.Requests, &edge.Errors); err != nil {
			return nil, fmt.Errorf("query: service map: %w", err)
		}
		out = append(out, edge)
	}
	return out, rows.Err()
}
