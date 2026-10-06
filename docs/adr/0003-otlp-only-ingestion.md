# 0003. OTLP is the only ingestion protocol

- Status: Accepted
- Date: 2026-10-06

## Context

Vendor lock-in is the problem obsrv exists to solve. Proprietary agents and SDKs are the main
source of that lock-in.

## Decision

obsrv only accepts OTLP (gRPC on 4317, HTTP on 4318). Other sources (Prometheus, syslog, files, …)
are bridged by the standard OpenTelemetry Collector. obsrv never ships its own SDK or agent and
never requires obsrv-specific attributes.

## Consequences

- Users can leave obsrv without re-instrumenting anything.
- We maintain one ingestion path instead of many.
- Users who are not on OpenTelemetry need a Collector in front of obsrv. We will document
  ready-to-use Collector configurations.
