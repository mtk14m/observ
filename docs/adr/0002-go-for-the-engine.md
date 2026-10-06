# 0002. Go for the engine

- Status: Accepted
- Date: 2026-10-06

## Context

We considered Rust (native Arrow, Parquet and DataFusion) and Go (OpenTelemetry ecosystem,
simplicity, and the team's strongest language). Both are fast enough. Neither choice has been
benchmarked on our workload yet.

## Decision

The whole V1 engine is written in Go. A component may be rewritten in Rust later only if:

1. profiling on a real workload shows it accounts for more than 30% of CPU;
2. reasonable Go optimisation has been exhausted;
3. a Rust prototype is more than 2× faster on the same benchmark.

## Consequences

- We iterate faster and debug more easily, and we can reuse `go.opentelemetry.io/collector/pdata`.
- There is no DataFusion in Go, so SQL execution uses embedded DuckDB (cgo). A dedicated ADR will cover this.
- Because the on-disk format is public, any future Rust component can coexist with the Go code without migration.
