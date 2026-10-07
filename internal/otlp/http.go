package otlp

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// DefaultMaxBodyBytes bounds the size of a request body, before and after
// decompression.
const DefaultMaxBodyBytes = 32 << 20

const (
	contentTypeProto = "application/x-protobuf"
	contentTypeJSON  = "application/json"
)

// HTTPOptions configures the OTLP/HTTP handler.
type HTTPOptions struct {
	// MaxBodyBytes defaults to DefaultMaxBodyBytes when zero.
	MaxBodyBytes int64
	// Token, when set, must be sent as "Authorization: Bearer <token>".
	Token string
}

// NewHTTPHandler returns an http.Handler implementing the OTLP/HTTP
// endpoints /v1/traces, /v1/metrics and /v1/logs.
func NewHTTPHandler(sink Sink, opts HTTPOptions) http.Handler {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = DefaultMaxBodyBytes
	}
	h := &httpHandler{maxBody: opts.MaxBodyBytes, token: opts.Token}
	mux := http.NewServeMux()
	mux.Handle("POST /v1/traces", h.endpoint(func(ctx context.Context, body []byte, json bool) (encoder, error) {
		req := ptraceotlp.NewExportRequest()
		if err := unmarshal(req, body, json); err != nil {
			return nil, err
		}
		return ptraceotlp.NewExportResponse(), sink.ConsumeTraces(ctx, req.Traces())
	}))
	mux.Handle("POST /v1/metrics", h.endpoint(func(ctx context.Context, body []byte, json bool) (encoder, error) {
		req := pmetricotlp.NewExportRequest()
		if err := unmarshal(req, body, json); err != nil {
			return nil, err
		}
		return pmetricotlp.NewExportResponse(), sink.ConsumeMetrics(ctx, req.Metrics())
	}))
	mux.Handle("POST /v1/logs", h.endpoint(func(ctx context.Context, body []byte, json bool) (encoder, error) {
		req := plogotlp.NewExportRequest()
		if err := unmarshal(req, body, json); err != nil {
			return nil, err
		}
		return plogotlp.NewExportResponse(), sink.ConsumeLogs(ctx, req.Logs())
	}))
	return mux
}

type unmarshaler interface {
	UnmarshalProto([]byte) error
	UnmarshalJSON([]byte) error
}

type encoder interface {
	MarshalProto() ([]byte, error)
	MarshalJSON() ([]byte, error)
}

// errDecode marks a malformed payload; it maps to 400 Bad Request.
var errDecode = errors.New("otlp: malformed payload")

func unmarshal(u unmarshaler, body []byte, json bool) error {
	var err error
	if json {
		err = u.UnmarshalJSON(body)
	} else {
		err = u.UnmarshalProto(body)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", errDecode, err)
	}
	return nil
}

// consumeFunc decodes body, hands it to the sink and returns the response.
type consumeFunc func(ctx context.Context, body []byte, json bool) (encoder, error)

type httpHandler struct {
	maxBody int64
	token   string
}

func (h *httpHandler) endpoint(consume consumeFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" && !validBearer(r.Header.Get("Authorization"), h.token) {
			json, _ := isJSON(r.Header.Get("Content-Type"))
			writeError(w, json, http.StatusUnauthorized, "missing or invalid ingest token")
			return
		}
		json, ok := isJSON(r.Header.Get("Content-Type"))
		if !ok {
			writeError(w, false, http.StatusUnsupportedMediaType, "unsupported content type, use application/x-protobuf or application/json")
			return
		}

		body, status, err := h.readBody(w, r)
		if err != nil {
			writeError(w, json, status, err.Error())
			return
		}

		resp, err := consume(r.Context(), body, json)
		switch {
		case errors.Is(err, errDecode):
			writeError(w, json, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrBackpressure):
			w.Header().Set("Retry-After", "1")
			writeError(w, json, http.StatusTooManyRequests, err.Error())
		case err != nil:
			w.Header().Set("Retry-After", "1")
			writeError(w, json, http.StatusServiceUnavailable, err.Error())
		default:
			writeResponse(w, json, resp)
		}
	})
}

// readBody reads the (optionally gzip-compressed) body, enforcing maxBody on
// both the compressed and decompressed sizes. It returns the HTTP status to
// use on failure.
func (h *httpHandler) readBody(w http.ResponseWriter, r *http.Request) ([]byte, int, error) {
	var src io.Reader = http.MaxBytesReader(w, r.Body, h.maxBody)
	switch r.Header.Get("Content-Encoding") {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(src)
		if err != nil {
			return nil, statusForReadError(err), fmt.Errorf("invalid gzip body: %w", err)
		}
		defer func() { _ = zr.Close() }()
		src = zr
	default:
		return nil, http.StatusUnsupportedMediaType, errors.New("unsupported content encoding, use gzip or none")
	}

	body, err := io.ReadAll(io.LimitReader(src, h.maxBody+1))
	if err != nil {
		return nil, statusForReadError(err), fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > h.maxBody {
		return nil, http.StatusRequestEntityTooLarge, errors.New("decompressed body too large")
	}
	return body, 0, nil
}

func statusForReadError(err error) int {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

// isJSON parses the Content-Type and reports whether the payload is JSON.
// ok is false for unsupported media types.
func isJSON(contentType string) (json, ok bool) {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, false
	}
	switch mt {
	case contentTypeProto:
		return false, true
	case contentTypeJSON:
		return true, true
	default:
		return false, false
	}
}

func writeResponse(w http.ResponseWriter, json bool, resp encoder) {
	var (
		b   []byte
		err error
	)
	if json {
		b, err = resp.MarshalJSON()
	} else {
		b, err = resp.MarshalProto()
	}
	if err != nil {
		writeError(w, json, http.StatusInternalServerError, "encode response: "+err.Error())
		return
	}
	writeBody(w, json, http.StatusOK, b)
}

// writeError writes a google.rpc.Status body, as required by OTLP/HTTP.
func writeError(w http.ResponseWriter, json bool, status int, msg string) {
	st := &spb.Status{Code: int32(grpcCode(status)), Message: msg}
	var (
		b   []byte
		err error
	)
	if json {
		b, err = protojson.Marshal(st)
	} else {
		b, err = proto.Marshal(st)
	}
	if err != nil {
		http.Error(w, msg, status)
		return
	}
	writeBody(w, json, status, b)
}

func writeBody(w http.ResponseWriter, json bool, status int, b []byte) {
	ct := contentTypeProto
	if json {
		ct = contentTypeJSON
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// grpcCode maps HTTP statuses to the closest gRPC status code.
func grpcCode(status int) int {
	switch status {
	case http.StatusBadRequest, http.StatusUnsupportedMediaType:
		return 3 // INVALID_ARGUMENT
	case http.StatusRequestEntityTooLarge, http.StatusTooManyRequests:
		return 8 // RESOURCE_EXHAUSTED
	case http.StatusServiceUnavailable:
		return 14 // UNAVAILABLE
	case http.StatusUnauthorized:
		return 16 // UNAUTHENTICATED
	default:
		return 13 // INTERNAL
	}
}
