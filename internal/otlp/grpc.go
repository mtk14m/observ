package otlp

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // registers the gzip decompressor
	"google.golang.org/grpc/status"
)

// RegisterGRPC registers the OTLP/gRPC trace, metric and log services on s.
func RegisterGRPC(s *grpc.Server, sink Sink) {
	ptraceotlp.RegisterGRPCServer(s, &traceServer{sink: sink})
	pmetricotlp.RegisterGRPCServer(s, &metricServer{sink: sink})
	plogotlp.RegisterGRPCServer(s, &logServer{sink: sink})
}

type traceServer struct {
	ptraceotlp.UnimplementedGRPCServer
	sink Sink
}

func (s *traceServer) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	return ptraceotlp.NewExportResponse(), grpcError(s.sink.ConsumeTraces(ctx, req.Traces()))
}

type metricServer struct {
	pmetricotlp.UnimplementedGRPCServer
	sink Sink
}

func (s *metricServer) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	return pmetricotlp.NewExportResponse(), grpcError(s.sink.ConsumeMetrics(ctx, req.Metrics()))
}

type logServer struct {
	plogotlp.UnimplementedGRPCServer
	sink Sink
}

func (s *logServer) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	return plogotlp.NewExportResponse(), grpcError(s.sink.ConsumeLogs(ctx, req.Logs()))
}

// grpcError maps sink errors to retryable gRPC codes, as OTLP requires.
func grpcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrBackpressure):
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Error(codes.Unavailable, err.Error())
	}
}
