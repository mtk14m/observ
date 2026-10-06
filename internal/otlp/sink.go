// Package otlp receives OpenTelemetry data over OTLP and hands it to a Sink.
package otlp

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// ErrBackpressure tells the receiver that the sink is temporarily full.
// Clients get a retryable "429 Too Many Requests" (or RESOURCE_EXHAUSTED).
var ErrBackpressure = errors.New("otlp: sink is applying backpressure")

// Sink consumes decoded telemetry. A nil error means the data is durably
// accepted and the client may be acknowledged.
type Sink interface {
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
	ConsumeLogs(ctx context.Context, ld plog.Logs) error
}
