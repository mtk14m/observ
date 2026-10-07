package query_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/mtk14n/obsrv/internal/compact"
	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore/cache"
	"github.com/mtk14n/obsrv/internal/objstore/s3"
	"github.com/mtk14n/obsrv/internal/query"
)

// TestEndToEndOnS3 ingests into a bucket, compacts it and queries it
// through the local cache, as obsrv does with -storage=s3.
func TestEndToEndOnS3(t *testing.T) {
	backend := s3mem.New()
	if err := backend.CreateBucket("telemetry"); err != nil {
		t.Fatal(err)
	}
	fake := gofakes3.New(backend).Server()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.Header.Get("Content-Length") == "" && r.ContentLength <= 0 {
			r.Header.Set("Content-Length", "0")
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()

	store, err := s3.New(t.Context(), s3.Config{
		Endpoint: strings.TrimPrefix(srv.URL, "http://"), Bucket: "telemetry", Prefix: "obsrv",
		AccessKey: "k", SecretKey: "s", Insecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	for i := range 2 { // two flushes: two files to compact
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "api")
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.SetTimestamp(at(time.Duration(i+1) * time.Minute))
		lr.Body().SetStr("stored in a bucket")
		if err := p.ConsumeLogs(t.Context(), ld); err != nil {
			t.Fatal(err)
		}
		if err := p.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	c := compact.New(compact.Options{Store: store, Now: func() time.Time { return base.Add(3 * time.Hour) }})
	if res, err := c.RunOnce(t.Context()); err != nil || res.FilesMerged != 2 {
		t.Fatalf("compaction = %+v, %v", res, err)
	}

	files, err := cache.New(cache.Options{Store: store, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	e, err := query.New(files)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Close() }()
	got, err := e.SearchLogs(t.Context(), query.LogQuery{TimeRange: window, Query: "bucket"})
	if err != nil || len(got) != 2 {
		t.Errorf("logs from S3 = %d, %v; want 2", len(got), err)
	}
}
