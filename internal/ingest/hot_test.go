package ingest_test

import (
	"os"
	"testing"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/pkg/schema"
)

func hotLogs(t *testing.T, p *ingest.Pipeline) []schema.Log {
	t.Helper()
	paths, err := p.HotFiles(layout.Logs)
	if err != nil {
		t.Fatalf("HotFiles: %v", err)
	}
	var rows []schema.Log
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		info, _ := f.Stat()
		got, err := parquet.Read[schema.Log](f, info.Size())
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, got...)
	}
	return rows
}

func TestHotFilesExposeUnflushedRows(t *testing.T) {
	p := newEnv(t).pipeline(t)
	if got := hotLogs(t, p); len(got) != 0 {
		t.Fatalf("hot rows before ingestion = %d", len(got))
	}

	_ = p.ConsumeLogs(t.Context(), logsAt("api", base, base))
	if got := hotLogs(t, p); len(got) != 2 {
		t.Fatalf("hot rows = %d, want 2", len(got))
	}

	// Once flushed, rows live in the store only: never twice.
	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := hotLogs(t, p); len(got) != 0 {
		t.Errorf("hot rows after flush = %d, want 0", len(got))
	}
}

func TestHotFilesFollowNewData(t *testing.T) {
	p := newEnv(t).pipeline(t, func(o *ingest.Options) { o.HotRefresh = -1 }) // always rebuild
	_ = p.ConsumeLogs(t.Context(), logsAt("api", base))
	_ = hotLogs(t, p)
	_ = p.ConsumeLogs(t.Context(), logsAt("api", base))
	if got := hotLogs(t, p); len(got) != 2 {
		t.Errorf("hot rows = %d, want 2", len(got))
	}
}

func TestHotFilesForEverySignal(t *testing.T) {
	p := newEnv(t).pipeline(t)
	_ = p.ConsumeTraces(t.Context(), spansAt(base))
	_ = p.ConsumeMetrics(t.Context(), gaugeAt(base))
	for _, dir := range []string{layout.Spans, layout.MetricPoints} {
		paths, err := p.HotFiles(dir)
		if err != nil || len(paths) != 1 {
			t.Errorf("HotFiles(%s) = %v, %v; want one file", dir, paths, err)
		}
	}
}
