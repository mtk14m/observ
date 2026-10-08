package query

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

// --- Compare ---------------------------------------------------------------

// CompareQuery compares a selection of records with the rest of the records
// matching Query in the range. The selection is either the errors, or the
// records of Window.
type CompareQuery struct {
	TimeRange
	// Signal is "logs" or "spans".
	Signal string
	Query  string
	Errors bool
	Window TimeRange
}

// CompareItem is an attribute value and how often it appears in the
// selection and in the baseline, as fractions between 0 and 1.
type CompareItem struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	Selection float64 `json:"selection"`
	Baseline  float64 `json:"baseline"`
}

// CompareResult lists the attribute values over-represented in the
// selection, most distinctive first.
type CompareResult struct {
	SelectionTotal int64         `json:"selection_total"`
	BaselineTotal  int64         `json:"baseline_total"`
	Items          []CompareItem `json:"items"`
}

const (
	compareLimit        = 12
	compareMinSelection = 0.2 // ignore values seen in under 20% of the selection
	compareMinGap       = 0.1
)

// Compare explains what distinguishes a selection from the rest: the
// attribute values (record and resource) that are over-represented in it.
func (e *Engine) Compare(ctx context.Context, q CompareQuery) (CompareResult, error) {
	var (
		dir, timeCol, errExpr string
		pred                  logsearch.Predicate
		err                   error
		extra                 string
	)
	switch q.Signal {
	case "logs":
		dir, timeCol = layout.Logs, "time_unix_nano"
		errExpr = "upper(severity_text) IN ('ERROR', 'FATAL')"
		pred, err = logsearch.Compile(q.Query)
	case "spans":
		dir, timeCol = layout.Spans, "start_time_unix_nano"
		errExpr = "status_code = 'Error'"
		pred, err = logsearch.CompileSpans(q.Query)
		extra = "UNION ALL SELECT sel, 'span.name', name FROM base"
	default:
		return CompareResult{}, fmt.Errorf("%w: signal must be logs or spans", logsearch.ErrSyntax)
	}
	if err != nil {
		return CompareResult{}, err
	}
	res := CompareResult{Items: []CompareItem{}}
	paths, err := e.files(ctx, dir, q.TimeRange)
	if err != nil || len(paths) == 0 {
		return res, err
	}

	selExpr := errExpr
	var selArgs []any
	if !q.Errors {
		selExpr = timeCol + " >= ? AND " + timeCol + " < ?"
		wFrom, wTo := q.Window.bounds()
		selArgs = []any{wFrom, wTo}
	}
	from, to := q.bounds()
	args := slices.Concat(selArgs, []any{from, to}, pred.Args)

	nameCol := "NULL::VARCHAR AS name"
	if q.Signal == "spans" {
		nameCol = "name"
	}
	rows, err := e.db.QueryContext(ctx, `
		WITH base AS (
			SELECT (`+selExpr+`) AS sel, attributes, resource_attributes, `+nameCol+`
			FROM `+source(paths)+`
			WHERE `+timeCol+` >= ? AND `+timeCol+` < ? AND (`+pred.Where+`)
		),
		kv AS (
			SELECT sel, e.key AS k, e.value AS v FROM (SELECT sel, unnest(map_entries(attributes)) AS e FROM base)
			UNION ALL
			SELECT sel, e.key, e.value FROM (SELECT sel, unnest(map_entries(resource_attributes)) AS e FROM base)
			`+extra+`
		)
		SELECT '' AS k, '' AS v, count(*) FILTER (WHERE sel), count(*) FILTER (WHERE NOT sel) FROM base
		UNION ALL
		SELECT k, v, count(*) FILTER (WHERE sel), count(*) FILTER (WHERE NOT sel) FROM kv GROUP BY k, v`, args...)
	if err != nil {
		return res, fmt.Errorf("query: compare: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type count struct {
		k, v     string
		sel, rst int64
	}
	var counts []count
	for rows.Next() {
		var c count
		if err := rows.Scan(&c.k, &c.v, &c.sel, &c.rst); err != nil {
			return res, fmt.Errorf("query: compare: %w", err)
		}
		if c.k == "" && c.v == "" {
			res.SelectionTotal, res.BaselineTotal = c.sel, c.rst
			continue
		}
		counts = append(counts, c)
	}
	if err := rows.Err(); err != nil || res.SelectionTotal == 0 {
		return res, err
	}
	for _, c := range counts {
		it := CompareItem{Key: c.k, Value: c.v, Selection: float64(c.sel) / float64(res.SelectionTotal)}
		if res.BaselineTotal > 0 {
			it.Baseline = float64(c.rst) / float64(res.BaselineTotal)
		}
		if it.Selection >= compareMinSelection && it.Selection-it.Baseline >= compareMinGap {
			res.Items = append(res.Items, it)
		}
	}
	// Most distinctive first; on ties the service comes first, as the most actionable hint.
	slices.SortFunc(res.Items, func(a, b CompareItem) int {
		return cmp.Or(
			cmp.Compare(b.Selection-b.Baseline, a.Selection-a.Baseline),
			cmp.Compare(boolRank(b.Key == "service.name"), boolRank(a.Key == "service.name")),
			cmp.Compare(a.Key, b.Key), cmp.Compare(a.Value, b.Value))
	})
	if len(res.Items) > compareLimit {
		res.Items = res.Items[:compareLimit]
	}
	return res, nil
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Issues ----------------------------------------------------------------

// Issue groups errors that share a service, a kind and a message template.
type Issue struct {
	ID      string `json:"id"`
	Service string `json:"service"`
	// Kind is "log" (error log records) or "span" (spans in error).
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	Example       string `json:"example"`
	Count         int64  `json:"count"`
	PreviousCount int64  `json:"previous_count"`
	FirstSeen     int64  `json:"first_seen"`
	LastSeen      int64  `json:"last_seen"`
	// Status is "new" (absent from the previous period), "rising" (at
	// least twice as frequent) or "ongoing".
	Status         string  `json:"status"`
	Buckets        []int64 `json:"buckets"`
	ExampleTraceID string  `json:"example_trace_id"`
}

var variableParts = regexp.MustCompile(`[0-9a-fA-F]{8,}|\d+(?:\.\d+)?`)

// Template replaces the variable parts of a message (numbers, hex IDs) with
// <*>, so that messages differing only by them group together.
func Template(message string) string { return variableParts.ReplaceAllString(message, "<*>") }

// Issues groups the errors of r into issues, compared with the period of the
// same length just before.
func (e *Engine) Issues(ctx context.Context, r TimeRange, buckets int) ([]Issue, error) {
	if buckets <= 0 {
		buckets = 24
	}
	prev := TimeRange{From: r.From.Add(-r.To.Sub(r.From)), To: r.To}
	step := max(r.To.Sub(r.From).Nanoseconds()/int64(buckets), 1)
	from, to := r.bounds()
	prevFrom := prev.From.UnixNano()

	issues := map[string]*Issue{}
	add := func(kind, dir, timeCol, errExpr, titleExpr string) error {
		paths, err := e.files(ctx, dir, prev)
		if err != nil || len(paths) == 0 {
			return err
		}
		rows, err := e.db.QueryContext(ctx, `
			SELECT service_name, `+titleExpr+` AS title,
			       CASE WHEN `+timeCol+` >= ? THEN (`+timeCol+` - ?) // ? ELSE -1 END AS bucket,
			       count(*), min(`+timeCol+`), max(`+timeCol+`), arg_max(`+titleExpr+`, `+timeCol+`), arg_max(trace_id, `+timeCol+`)
			FROM `+source(paths)+`
			WHERE `+timeCol+` >= ? AND `+timeCol+` < ? AND `+errExpr+`
			GROUP BY ALL`, from, from, step, prevFrom, to)
		if err != nil {
			return fmt.Errorf("query: issues: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var service, title, example, traceID string
			var bucket, count, first, last int64
			if err := rows.Scan(&service, &title, &bucket, &count, &first, &last, &example, &traceID); err != nil {
				return fmt.Errorf("query: issues: %w", err)
			}
			tpl := Template(title)
			sum := sha256.Sum256([]byte(service + "|" + kind + "|" + tpl))
			id := hex.EncodeToString(sum[:6])
			is := issues[id]
			if is == nil {
				is = &Issue{ID: id, Service: service, Kind: kind, Title: tpl, Buckets: make([]int64, buckets)}
				issues[id] = is
			}
			if bucket < 0 {
				is.PreviousCount += count
				continue
			}
			is.Count += count
			if b := min(bucket, int64(buckets-1)); b >= 0 {
				is.Buckets[b] += count
			}
			if is.FirstSeen == 0 || first < is.FirstSeen {
				is.FirstSeen = first
			}
			if last >= is.LastSeen {
				is.LastSeen, is.Example = last, example
				if traceID != "" {
					is.ExampleTraceID = traceID
				}
			}
		}
		return rows.Err()
	}
	if err := add("log", layout.Logs, "time_unix_nano", "upper(severity_text) IN ('ERROR', 'FATAL')", "body"); err != nil {
		return nil, err
	}
	if err := add("span", layout.Spans, "start_time_unix_nano", "status_code = 'Error'",
		"CASE WHEN status_message = '' THEN name ELSE name || ': ' || status_message END"); err != nil {
		return nil, err
	}

	out := make([]Issue, 0, len(issues))
	for _, is := range issues {
		if is.Count == 0 {
			continue
		}
		switch {
		case is.PreviousCount == 0:
			is.Status = "new"
		case is.Count >= 2*is.PreviousCount:
			is.Status = "rising"
		default:
			is.Status = "ongoing"
		}
		out = append(out, *is)
	}
	slices.SortFunc(out, func(a, b Issue) int { return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}

// --- Deployments -----------------------------------------------------------

// Deployment is a new version of a service first seen in the range.
type Deployment struct {
	Service  string `json:"service"`
	Version  string `json:"version"`
	Previous string `json:"previous"`
	At       int64  `json:"at"`
}

// deploymentLookback is how far back obsrv looks for a previous version.
const deploymentLookback = 24 * time.Hour

// Deployments lists the versions (service.version) that services started
// reporting within r, after reporting another version before it.
func (e *Engine) Deployments(ctx context.Context, r TimeRange) ([]Deployment, error) {
	scan := TimeRange{From: r.From.Add(-deploymentLookback), To: r.To}
	paths, err := e.files(ctx, layout.Spans, scan)
	if err != nil || len(paths) == 0 {
		return []Deployment{}, err
	}
	from, to := scan.bounds()
	rows, err := e.db.QueryContext(ctx, `
		SELECT service_name, resource_attributes['service.version'] AS version, min(start_time_unix_nano) AS first
		FROM `+source(paths)+`
		WHERE start_time_unix_nano >= ? AND start_time_unix_nano < ? AND resource_attributes['service.version'] IS NOT NULL
		GROUP BY ALL ORDER BY service_name, first`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query: deployments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []Deployment{}
	var service, previous string
	for rows.Next() {
		var svc, version string
		var first int64
		if err := rows.Scan(&svc, &version, &first); err != nil {
			return nil, fmt.Errorf("query: deployments: %w", err)
		}
		if svc != service {
			service, previous = svc, ""
		}
		if first >= r.From.UnixNano() && previous != "" && version != previous {
			out = append(out, Deployment{Service: svc, Version: version, Previous: previous, At: first})
		}
		previous = version
	}
	slices.SortFunc(out, func(a, b Deployment) int { return cmp.Compare(b.At, a.At) })
	return out, rows.Err()
}
