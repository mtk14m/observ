package compact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/compact"
	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/pkg/schema"
)

var now = time.Date(2026, 10, 7, 15, 30, 0, 0, time.UTC)

func newStore(t *testing.T) objstore.ObjectStore {
	t.Helper()
	s, err := fs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// putLogs writes one file of logs at the given times and returns its key.
func putLogs(t *testing.T, s objstore.ObjectStore, service string, times ...time.Time) string {
	t.Helper()
	rows := make([]schema.Log, len(times))
	for i, ts := range times {
		rows[i] = schema.Log{TimeUnixNano: ts.UnixNano(), ServiceName: service, Body: service}
	}
	var buf bytes.Buffer
	if err := parquet.Write(&buf, rows); err != nil {
		t.Fatal(err)
	}
	key := layout.NewKey(layout.Logs, times[0])
	if err := s.Put(context.Background(), key, &buf); err != nil {
		t.Fatal(err)
	}
	return key
}

func keys(t *testing.T, s objstore.ObjectStore, prefix string) []string {
	t.Helper()
	infos, err := s.List(context.Background(), prefix)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range infos {
		out = append(out, i.Key)
	}
	return out
}

func readLogs(t *testing.T, s objstore.ObjectStore, prefix string) []schema.Log {
	t.Helper()
	var rows []schema.Log
	for _, k := range keys(t, s, prefix) {
		rc, err := s.Get(context.Background(), k)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		got, err := parquet.Read[schema.Log](bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, got...)
	}
	return rows
}

func newCompactor(s objstore.ObjectStore, mutate ...func(*compact.Options)) *compact.Compactor {
	opts := compact.Options{Store: s, Now: func() time.Time { return now }}
	for _, m := range mutate {
		m(&opts)
	}
	return compact.New(opts)
}

var closedHour = time.Date(2026, 10, 7, 13, 0, 0, 0, time.UTC)

func TestMergesTheFilesOfAClosedHour(t *testing.T) {
	s := newStore(t)
	putLogs(t, s, "web", closedHour.Add(3*time.Minute))
	putLogs(t, s, "api", closedHour.Add(1*time.Minute))
	putLogs(t, s, "web", closedHour.Add(2*time.Minute))

	res, err := newCompactor(s).RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	prefix := "v1/logs/date=2026-10-07/hour=13/"
	if got := keys(t, s, prefix); len(got) != 1 {
		t.Fatalf("files after compaction = %v, want 1", got)
	}
	rows := readLogs(t, s, prefix)
	var order []string
	for _, r := range rows {
		order = append(order, r.ServiceName)
	}
	if !slices.Equal(order, []string{"api", "web", "web"}) || rows[1].TimeUnixNano > rows[2].TimeUnixNano {
		t.Errorf("rows not merged and sorted: %v", order)
	}
	if res.FilesMerged != 3 || res.FilesWritten != 1 {
		t.Errorf("result = %+v", res)
	}
	if got := keys(t, s, "v1/_compaction/"); len(got) != 0 {
		t.Errorf("leftover manifests: %v", got)
	}
}

func TestLeavesOpenHoursAndSingleFilesAlone(t *testing.T) {
	s := newStore(t)
	open1 := putLogs(t, s, "a", now.Add(-time.Minute))
	open2 := putLogs(t, s, "b", now.Add(-2*time.Minute))
	// The previous hour ended 30 minutes ago but is still within the grace period.
	recent1 := putLogs(t, s, "c", now.Truncate(time.Hour).Add(-time.Minute))
	recent2 := putLogs(t, s, "d", now.Truncate(time.Hour).Add(-2*time.Minute))
	single := putLogs(t, s, "e", closedHour)

	if _, err := newCompactor(s, func(o *compact.Options) { o.Grace = time.Hour }).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := []string{open1, open2, recent1, recent2, single}
	slices.Sort(want)
	if got := keys(t, s, "v1/logs/"); !slices.Equal(got, want) {
		t.Errorf("files = %v, want unchanged %v", got, want)
	}
}

func TestSplitsLargePartitionsBySize(t *testing.T) {
	s := newStore(t)
	for range 4 {
		putLogs(t, s, "a", closedHour)
	}
	c := newCompactor(s, func(o *compact.Options) { o.TargetBytes = 1 }) // every file "fills" an output
	if _, err := c.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := readLogs(t, s, "v1/logs/"); len(got) != 4 {
		t.Errorf("rows = %d, want 4", len(got))
	}
}

func TestRetentionDeletesOldPartitions(t *testing.T) {
	s := newStore(t)
	putLogs(t, s, "old", now.Add(-50*time.Hour))
	keep := putLogs(t, s, "new", now.Add(-10*time.Hour))

	res, err := newCompactor(s, func(o *compact.Options) { o.Retention = 48 * time.Hour }).RunOnce(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := keys(t, s, "v1/"); !slices.Equal(got, []string{keep}) {
		t.Errorf("files = %v, want only %v", got, keep)
	}
	if res.FilesExpired != 1 {
		t.Errorf("FilesExpired = %d, want 1", res.FilesExpired)
	}
}

func TestRecoversFromACrashAfterWritingTheOutput(t *testing.T) {
	s := newStore(t)
	a := putLogs(t, s, "a", closedHour)
	b := putLogs(t, s, "b", closedHour)
	// The previous run wrote the merged file and its manifest, then crashed
	// before deleting the sources.
	out := putLogs(t, s, "merged", closedHour, closedHour)
	writeManifest(t, s, out, a, b)

	if _, err := newCompactor(s).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	rows := readLogs(t, s, "v1/logs/")
	if len(rows) != 2 || rows[0].ServiceName != "merged" {
		t.Errorf("rows = %+v, want only the 2 merged rows (no duplicates)", rows)
	}
}

func TestRecoversFromACrashBeforeWritingTheOutput(t *testing.T) {
	s := newStore(t)
	a := putLogs(t, s, "a", closedHour)
	b := putLogs(t, s, "b", closedHour)
	writeManifest(t, s, layout.NewKey(layout.Logs, closedHour), a, b) // output never written

	if _, err := newCompactor(s).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if rows := readLogs(t, s, "v1/logs/"); len(rows) != 2 {
		t.Errorf("rows = %d, want 2 (sources kept, then compacted)", len(rows))
	}
	if got := keys(t, s, "v1/_compaction/"); len(got) != 0 {
		t.Errorf("leftover manifests: %v", got)
	}
}

func TestCompactsEverySignal(t *testing.T) {
	s := newStore(t)
	for _, dir := range []string{layout.Spans, layout.MetricPoints} {
		for range 2 {
			var buf bytes.Buffer
			var err error
			if dir == layout.Spans {
				err = parquet.Write(&buf, []schema.Span{{StartTimeUnixNano: closedHour.UnixNano()}})
			} else {
				err = parquet.Write(&buf, []schema.MetricPoint{{TimeUnixNano: closedHour.UnixNano()}})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Put(t.Context(), layout.NewKey(dir, closedHour), &buf); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := newCompactor(s).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{layout.Spans, layout.MetricPoints} {
		if got := keys(t, s, layout.Prefix(dir)); len(got) != 1 {
			t.Errorf("%s files = %v, want 1", dir, got)
		}
	}
}

func writeManifest(t *testing.T, s objstore.ObjectStore, output string, sources ...string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"output": output, "sources": sources})
	key := "v1/_compaction/" + strings.ReplaceAll(output[strings.LastIndex(output, "/")+1:], ".parquet", ".json")
	if err := s.Put(context.Background(), key, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}
