package otlp_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mtk14n/obsrv/internal/otlp"
)

func dialGRPC(t *testing.T, sink otlp.Sink) *grpc.ClientConn {
	t.Helper()
	return dialGRPCWith(t, sink)
}

func dialGRPCWith(t *testing.T, sink otlp.Sink, opts ...grpc.ServerOption) *grpc.ClientConn {
	t.Helper()
	ln := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(opts...)
	otlp.RegisterGRPC(srv, sink)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return ln.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestGRPCAcceptsEverySignal(t *testing.T) {
	sigs := signals(t)
	sink := &recordingSink{}
	conn := dialGRPC(t, sink)
	ctx := t.Context()

	traces := ptraceotlp.NewExportRequest()
	if err := traces.UnmarshalProto(sigs[0].body(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := ptraceotlp.NewGRPCClient(conn).Export(ctx, traces); err != nil {
		t.Fatalf("traces Export: %v", err)
	}

	metrics := pmetricotlp.NewExportRequest()
	if err := metrics.UnmarshalProto(sigs[1].body(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := pmetricotlp.NewGRPCClient(conn).Export(ctx, metrics); err != nil {
		t.Fatalf("metrics Export: %v", err)
	}

	logs := plogotlp.NewExportRequest()
	if err := logs.UnmarshalProto(sigs[2].body(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := plogotlp.NewGRPCClient(conn).Export(ctx, logs); err != nil {
		t.Fatalf("logs Export: %v", err)
	}

	if sink.spans != 2 || sink.points != 1 || sink.logs != 3 {
		t.Errorf("sink received spans=%d points=%d logs=%d, want 2/1/3", sink.spans, sink.points, sink.logs)
	}
}

func TestGRPCErrorCodes(t *testing.T) {
	tests := []struct {
		name    string
		sinkErr error
		want    codes.Code
	}{
		{"backpressure is retryable resource exhausted", otlp.ErrBackpressure, codes.ResourceExhausted},
		{"other failures are retryable unavailable", errors.New("disk on fire"), codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := dialGRPC(t, &recordingSink{err: tt.sinkErr})
			_, err := plogotlp.NewGRPCClient(conn).Export(t.Context(), plogotlp.NewExportRequest())
			if got := status.Code(err); got != tt.want {
				t.Errorf("code = %v, want %v (err=%v)", got, tt.want, err)
			}
		})
	}
}
