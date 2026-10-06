package query_test

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/query"
)

func TestUnflushedDataIsQueryableOnce(t *testing.T) {
	store, _ := fs.New(t.TempDir())
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	e, err := query.New(store, query.WithHot(p))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "api")
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(at(time.Minute))
	lr.Body().SetStr("just arrived")
	if err := p.ConsumeLogs(t.Context(), ld); err != nil {
		t.Fatal(err)
	}

	got, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Body != "just arrived" {
		t.Fatalf("before flush: %+v, want the unflushed record", got)
	}

	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window}); len(got) != 1 {
		t.Errorf("after flush: %d records, want exactly 1", len(got))
	}
}
