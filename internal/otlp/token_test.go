package otlp_test

import (
	"net/http"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mtk14n/obsrv/internal/otlp"
)

func TestHTTPRequiresTheIngestToken(t *testing.T) {
	sink := &recordingSink{}
	srv := newServer(t, sink, otlp.HTTPOptions{Token: "s3cret"})
	body := signals(t)[2].body(false)

	for name, header := range map[string]string{"missing": "", "wrong": "Bearer nope", "not bearer": "s3cret"} {
		h := map[string]string{}
		if header != "" {
			h["Authorization"] = header
		}
		if resp := post(t, srv.URL+"/v1/logs", "application/x-protobuf", body, h); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s token: status %d, want 401", name, resp.StatusCode)
		}
	}
	resp := post(t, srv.URL+"/v1/logs", "application/x-protobuf", body, map[string]string{"Authorization": "Bearer s3cret"})
	if resp.StatusCode != http.StatusOK || sink.logs != 3 {
		t.Errorf("valid token: status %d, logs %d", resp.StatusCode, sink.logs)
	}
}

func TestGRPCRequiresTheIngestToken(t *testing.T) {
	conn := dialGRPCWith(t, &recordingSink{}, grpc.UnaryInterceptor(otlp.TokenInterceptor("s3cret")))
	client := plogotlp.NewGRPCClient(conn)

	_, err := client.Export(t.Context(), plogotlp.NewExportRequest())
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("no token: %v, want Unauthenticated", err)
	}
	ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", "Bearer s3cret")
	if _, err := client.Export(ctx, plogotlp.NewExportRequest()); err != nil {
		t.Errorf("valid token: %v", err)
	}
}
