// Package exit_test is the "exit test": it proves that the data written by
// obsrv can be read with plain SQL by a standard tool (DuckDB), using only
// the public schema and layout, without any obsrv code on the read path.
//
// If this test breaks, users can no longer leave obsrv freely.
package exit_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
)

func TestDataIsReadableWithoutObsrv(t *testing.T) {
	root := t.TempDir()
	writeOneOfEach(t, root)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	glob := func(dir string) string { return filepath.Join(root, "v1", dir, "**", "*.parquet") }
	checks := []struct {
		name  string
		query string
		want  string
	}{
		{"logs", `SELECT body FROM read_parquet(?) WHERE service_name = 'shop'`, "hello"},
		{"spans", `SELECT name FROM read_parquet(?) WHERE attributes['http.route'] = '/buy'`, "GET /buy"},
		{"metric points", `SELECT metric_name FROM read_parquet(?) WHERE value = 42`, "stock"},
	}
	dirs := []string{"logs", "spans", "metric_points"}
	for i, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			var got string
			if err := db.QueryRowContext(t.Context(), c.query, glob(dirs[i])).Scan(&got); err != nil {
				t.Fatalf("query: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func writeOneOfEach(t *testing.T, root string) {
	t.Helper()
	store, err := fs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	now := pcommon.NewTimestampFromTime(time.Now())

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "shop")
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(now)
	lr.Body().SetStr("hello")

	td := ptrace.NewTraces()
	s := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName("GET /buy")
	s.SetStartTimestamp(now)
	s.SetEndTimestamp(now)
	s.Attributes().PutStr("http.route", "/buy")

	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("stock")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntValue(42)

	ctx := t.Context()
	if err := p.ConsumeLogs(ctx, ld); err != nil {
		t.Fatal(err)
	}
	if err := p.ConsumeTraces(ctx, td); err != nil {
		t.Fatal(err)
	}
	if err := p.ConsumeMetrics(ctx, md); err != nil {
		t.Fatal(err)
	}
	if err := p.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}
