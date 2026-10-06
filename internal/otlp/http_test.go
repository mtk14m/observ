package otlp_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mtk14n/obsrv/internal/otlp"
)

// recordingSink counts what it receives and can be told to fail.
type recordingSink struct {
	mu                  sync.Mutex
	spans, points, logs int
	err                 error
}

func (s *recordingSink) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans += td.SpanCount()
	return s.err
}

func (s *recordingSink) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points += md.DataPointCount()
	return s.err
}

func (s *recordingSink) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs += ld.LogRecordCount()
	return s.err
}

// signalCase describes one OTLP signal endpoint.
type signalCase struct {
	path     string
	body     func(json bool) []byte // a valid request body
	received func(s *recordingSink) int
	want     int
}

func signals(t *testing.T) []signalCase {
	t.Helper()
	return []signalCase{
		{
			path: "/v1/traces",
			body: func(json bool) []byte {
				td := ptrace.NewTraces()
				ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
				ss.Spans().AppendEmpty().SetName("a")
				ss.Spans().AppendEmpty().SetName("b")
				return marshal(t, ptraceotlp.NewExportRequestFromTraces(td), json)
			},
			received: func(s *recordingSink) int { return s.spans },
			want:     2,
		},
		{
			path: "/v1/metrics",
			body: func(json bool) []byte {
				md := pmetric.NewMetrics()
				m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				m.SetName("cpu")
				m.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(0.5)
				return marshal(t, pmetricotlp.NewExportRequestFromMetrics(md), json)
			},
			received: func(s *recordingSink) int { return s.points },
			want:     1,
		},
		{
			path: "/v1/logs",
			body: func(json bool) []byte {
				ld := plog.NewLogs()
				lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
				for range 3 {
					lr.AppendEmpty().Body().SetStr("hello")
				}
				return marshal(t, plogotlp.NewExportRequestFromLogs(ld), json)
			},
			received: func(s *recordingSink) int { return s.logs },
			want:     3,
		},
	}
}

type marshaler interface {
	MarshalProto() ([]byte, error)
	MarshalJSON() ([]byte, error)
}

func marshal(t *testing.T, m marshaler, json bool) []byte {
	t.Helper()
	var (
		b   []byte
		err error
	)
	if json {
		b, err = m.MarshalJSON()
	} else {
		b, err = m.MarshalProto()
	}
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func newServer(t *testing.T, sink otlp.Sink, opts otlp.HTTPOptions) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(otlp.NewHTTPHandler(sink, opts))
	t.Cleanup(srv.Close)
	return srv
}

// result is a fully read HTTP response.
type result struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func post(t *testing.T, url, contentType string, body []byte, headers map[string]string) result {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{StatusCode: resp.StatusCode, Header: resp.Header, Body: b}
}

func TestHTTPAcceptsEverySignal(t *testing.T) {
	encodings := []struct {
		name        string
		contentType string
		json        bool
	}{
		{"protobuf", "application/x-protobuf", false},
		{"json", "application/json", true},
		{"json with charset", "application/json; charset=utf-8", true},
	}
	for _, sig := range signals(t) {
		for _, enc := range encodings {
			t.Run(sig.path+"/"+enc.name, func(t *testing.T) {
				sink := &recordingSink{}
				srv := newServer(t, sink, otlp.HTTPOptions{})

				resp := post(t, srv.URL+sig.path, enc.contentType, sig.body(enc.json), nil)

				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}
				if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(enc.contentType, got) {
					t.Errorf("response Content-Type = %q, want it to match request %q", got, enc.contentType)
				}
				if got := sig.received(sink); got != sig.want {
					t.Errorf("sink received %d items, want %d", got, sig.want)
				}
			})
		}
	}
}

func TestHTTPAcceptsGzip(t *testing.T) {
	for _, sig := range signals(t) {
		t.Run(sig.path, func(t *testing.T) {
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			if _, err := zw.Write(sig.body(false)); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			sink := &recordingSink{}
			srv := newServer(t, sink, otlp.HTTPOptions{})

			resp := post(t, srv.URL+sig.path, "application/x-protobuf", buf.Bytes(),
				map[string]string{"Content-Encoding": "gzip"})

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if got := sig.received(sink); got != sig.want {
				t.Errorf("sink received %d items, want %d", got, sig.want)
			}
		})
	}
}

func TestHTTPErrors(t *testing.T) {
	valid := signals(t)[0].body(false)
	tests := []struct {
		name        string
		contentType string
		encoding    string
		body        []byte
		sinkErr     error
		maxBody     int64
		wantStatus  int
		wantRetry   bool
	}{
		{name: "unsupported content type", contentType: "text/plain", body: valid, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unsupported content encoding", contentType: "application/x-protobuf", encoding: "br", body: valid, wantStatus: http.StatusUnsupportedMediaType},
		{name: "malformed protobuf", contentType: "application/x-protobuf", body: []byte{0xff, 0xff, 0xff}, wantStatus: http.StatusBadRequest},
		{name: "malformed json", contentType: "application/json", body: []byte("{not json"), wantStatus: http.StatusBadRequest},
		{name: "corrupt gzip", contentType: "application/x-protobuf", encoding: "gzip", body: []byte("nope"), wantStatus: http.StatusBadRequest},
		{name: "body too large", contentType: "application/x-protobuf", body: valid, maxBody: 4, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "backpressure", contentType: "application/x-protobuf", body: valid, sinkErr: otlp.ErrBackpressure, wantStatus: http.StatusTooManyRequests, wantRetry: true},
		{name: "wrapped backpressure", contentType: "application/x-protobuf", body: valid, sinkErr: errors.Join(errors.New("wal full"), otlp.ErrBackpressure), wantStatus: http.StatusTooManyRequests, wantRetry: true},
		{name: "sink failure", contentType: "application/x-protobuf", body: valid, sinkErr: errors.New("disk on fire"), wantStatus: http.StatusServiceUnavailable, wantRetry: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, &recordingSink{err: tt.sinkErr}, otlp.HTTPOptions{MaxBodyBytes: tt.maxBody})
			headers := map[string]string{}
			if tt.encoding != "" {
				headers["Content-Encoding"] = tt.encoding
			}

			resp := post(t, srv.URL+"/v1/traces", tt.contentType, tt.body, headers)

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := resp.Header.Get("Retry-After") != ""; got != tt.wantRetry {
				t.Errorf("Retry-After present = %v, want %v", got, tt.wantRetry)
			}
			// OTLP/HTTP: error bodies are a google.rpc.Status message.
			if resp.Header.Get("Content-Type") == "application/x-protobuf" {
				var st spb.Status
				if err := proto.Unmarshal(resp.Body, &st); err != nil || st.GetMessage() == "" {
					t.Errorf("error body is not a Status with a message: err=%v, status=%v", err, &st)
				}
			}
		})
	}
}

func TestHTTPRejectsNonPost(t *testing.T) {
	srv := newServer(t, &recordingSink{}, otlp.HTTPOptions{})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPUnknownPath(t *testing.T) {
	srv := newServer(t, &recordingSink{}, otlp.HTTPOptions{})
	resp := post(t, srv.URL+"/v1/profiles", "application/x-protobuf", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
