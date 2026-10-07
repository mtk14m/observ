# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- OTLP ingestion over HTTP (protobuf, JSON, gzip) and gRPC for traces, metrics and logs.
- Durable ingestion: write-ahead log with group commit, crash recovery and backpressure.
- Storage as ZSTD Parquet files with a public, versioned schema (`pkg/schema`), partitioned by
  signal, date and hour, on the local filesystem.
- Queries with an embedded DuckDB: service request/error/latency statistics, log search with a
  simple syntax, log volume histogram, trace search, trace by ID, metric listing and metric
  queries (rates, counter resets, histogram percentiles, group by).
- JSON API under `/api/v1`.
- Web UI (Vue 3) embedded in the binary: services, logs, traces with a waterfall, metrics
  explorer.
- Exit test proving the stored data is readable with plain DuckDB SQL.
- `demo-shop` example and a Docker Compose demo (`make demo`).
- Service page: request, error and latency charts, operations, callers and callees.
- Service map on the Services page, derived from cross-service parent/child spans.
- Compaction of closed hours and a `-retention` flag (7 days by default).
- Unflushed telemetry is queryable immediately.
- Free-text log search also matches attribute values.
- Benchmark harness (`make bench`) and first results in `bench/RESULTS.md`.
- "Past 30 minutes" time range preset and a favicon for the web UI.
- S3-compatible storage (`-storage=s3`) with a local query cache, and a MinIO demo (`make demo-s3`).
- Every flag can be set with an `OBSRV_*` environment variable.
- Alerting: rules on log counts or metrics (optionally per group), with a waiting period, evaluated
  every `-alert-interval`; notifications to Slack and webhooks; an alerts page with live preview,
  channels and event history. Metadata is stored in an embedded SQLite database.

### Changed

- Parquet files use delta and dictionary encodings where data repeats (smaller files, same
  logical schema).
