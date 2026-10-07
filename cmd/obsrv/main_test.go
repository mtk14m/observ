package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtk14n/obsrv/internal/auth"
	"github.com/mtk14n/obsrv/internal/metadata"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-version"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(out.String(), "obsrv ") {
		t.Errorf("output = %q, want it to start with %q", out.String(), "obsrv ")
	}
}

func TestUnknownFlagFails(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-nope"}, &out); err == nil {
		t.Error("run with unknown flag returned nil error")
	}
}

func TestHealthEndpoints(t *testing.T) {
	srv := httptest.NewServer(newAPIHandler(nil, nil, testAuth(t)))
	defer srv.Close()

	for _, path := range []string{"/healthz", "/readyz", "/", "/logs"} {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestServeStopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-otlp-grpc-addr", "127.0.0.1:0", "-otlp-http-addr", "127.0.0.1:0", "-http-addr", "127.0.0.1:0", "-data-dir", dir}, &bytes.Buffer{})
	}()
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run returned %v after cancel, want nil", err)
	}
}

func testAuth(t *testing.T) *auth.Handler {
	t.Helper()
	db, err := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"))
	if err == nil {
		err = metadata.Migrate(t.Context(), db, "auth", auth.Migrations)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return auth.NewHandler(auth.NewStore(db, auth.StoreOptions{FastHashing: true}), auth.HandlerOptions{})
}

func TestAPIRequiresSignIn(t *testing.T) {
	srv := httptest.NewServer(newAPIHandler(nil, nil, testAuth(t)))
	defer srv.Close()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/v1/services", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous API call = %d, want 401", resp.StatusCode)
	}
}
