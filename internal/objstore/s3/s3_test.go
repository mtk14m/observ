package s3_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/objstoretest"
	"github.com/mtk14n/obsrv/internal/objstore/s3"
)

// fakeS3 starts an in-memory S3 server with one bucket.
func fakeS3(t *testing.T, bucket string) s3.Config {
	t.Helper()
	backend := s3mem.New()
	if err := backend.CreateBucket(bucket); err != nil {
		t.Fatal(err)
	}
	fake := gofakes3.New(backend).Server()
	// gofakes3 rejects empty uploads that carry no Content-Length header,
	// which Go's HTTP client omits for empty bodies. Real S3 and MinIO
	// accept them (see TestConformanceAgainstRealS3), so patch the request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.Header.Get("Content-Length") == "" && r.ContentLength <= 0 {
			r.Header.Set("Content-Length", "0")
		}
		fake.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return s3.Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    bucket,
		AccessKey: "test",
		SecretKey: "test",
		Insecure:  true,
	}
}

var seq atomic.Int64

func TestConformance(t *testing.T) {
	objstoretest.Run(t, func(t *testing.T) objstore.ObjectStore {
		s, err := s3.New(t.Context(), fakeS3(t, "obsrv"))
		if err != nil {
			t.Fatalf("s3.New: %v", err)
		}
		return s
	})
}

// TestConformanceAgainstRealS3 runs the suite against a real S3-compatible
// server when OBSRV_TEST_S3_ENDPOINT is set, for example with MinIO:
//
//	docker run -p 9000:9000 minio/minio server /data
//	OBSRV_TEST_S3_ENDPOINT=localhost:9000 OBSRV_TEST_S3_BUCKET=test go test ./internal/objstore/s3/
func TestConformanceAgainstRealS3(t *testing.T) {
	endpoint := os.Getenv("OBSRV_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("OBSRV_TEST_S3_ENDPOINT is not set")
	}
	objstoretest.Run(t, func(t *testing.T) objstore.ObjectStore {
		s, err := s3.New(t.Context(), s3.Config{
			Endpoint:  endpoint,
			Bucket:    os.Getenv("OBSRV_TEST_S3_BUCKET"),
			Prefix:    fmt.Sprintf("conformance-%d-%d", os.Getpid(), seq.Add(1)),
			AccessKey: envOr("OBSRV_TEST_S3_ACCESS_KEY", "minioadmin"),
			SecretKey: envOr("OBSRV_TEST_S3_SECRET_KEY", "minioadmin"),
			Insecure:  true,
		})
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestPrefixIsolatesObjects(t *testing.T) {
	cfg := fakeS3(t, "shared")
	raw, err := s3.New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Prefix = "team-a/obsrv"
	scoped, err := s3.New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := scoped.Put(t.Context(), "v1/logs/x.parquet", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	// The object lives under the prefix in the bucket...
	rc, err := raw.Get(t.Context(), "team-a/obsrv/v1/logs/x.parquet")
	if err != nil {
		t.Fatalf("object not stored under the prefix: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "data" {
		t.Errorf("content = %q", b)
	}
	// ...and the scoped store sees keys without it.
	infos, err := scoped.List(t.Context(), "")
	if err != nil || len(infos) != 1 || infos[0].Key != "v1/logs/x.parquet" {
		t.Errorf("List = %+v, %v", infos, err)
	}
}

func TestNewFailsForMissingBucket(t *testing.T) {
	cfg := fakeS3(t, "exists")
	cfg.Bucket = "missing"
	if _, err := s3.New(t.Context(), cfg); err == nil {
		t.Error("New with a missing bucket returned nil error")
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
