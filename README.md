# obsrv

**Logs, metrics and traces in a single container. OpenTelemetry-native. Your data stays yours.**

[![CI](https://github.com/mtk14n/obsrv/actions/workflows/ci.yml/badge.svg)](https://github.com/mtk14n/obsrv/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> **Status: early development.** obsrv is not usable in production yet. APIs, storage format and
> configuration will change without notice until v1.0. Follow the [roadmap](docs/ROADMAP.md).

## Why obsrv

- **All-in-one.** Ingestion, storage, query, UI, dashboards and alerting ship as one binary in one container.
  You don't need Postgres, ClickHouse, Kafka or Grafana.
- **OpenTelemetry-native.** OTLP is the only way in, so you use the official OpenTelemetry SDKs and Collector.
  There is no proprietary agent or SDK, ever.
- **Simple to query.** A visual query builder, SQL for power users and a simple log search syntax.
  You don't need to learn PromQL.
- **No lock-in.** Data is stored as Apache Parquet with a [public, versioned schema](docs/ARCHITECTURE.md#4-stockage--le-contrat-public),
  on local disk or in your own S3 bucket. You can read it with DuckDB, Spark or anything else, even when obsrv is not running.

## Try the demo

You need Docker. This starts obsrv and a small shop made of four services (frontend, checkout,
inventory, payment) instrumented with the official OpenTelemetry SDK:

```sh
make demo        # or: docker compose -f deploy/demo/docker-compose.yml up --build
```

Open <http://localhost:8080>. Data appears within about 15 seconds. Things to try:

1. **Services**: checkout has around 9% errors and a slow p95. Click it.
2. **Traces**: tick *Errors only*, open a trace and look at the waterfall: the payment call failed.
   The logs of that trace are listed under the waterfall.
3. **Logs**: search `level:error`, or `service:payment "refused"`. Click a line to see its
   attributes and jump to its trace.
4. **Metrics**: pick `http.server.request.duration`, show `p95` by `service.name`.
5. **Leave whenever you want**: the data is plain Parquet. Query it with DuckDB, without obsrv:

   ```sh
   docker compose -f deploy/demo/docker-compose.yml cp obsrv:/data/store ./obsrv-data
   duckdb -c "SELECT service_name, count(*) FROM './obsrv-data/v1/spans/**/*.parquet' GROUP BY 1"
   ```

Stop and delete everything with `make demo-down`.

## Send your own telemetry

Run obsrv (`docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 -v obsrv:/data <image>`, or
`make run` from source), then point any OpenTelemetry SDK or Collector at it:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318   # OTLP/HTTP, or :4317 for OTLP/gRPC
```

## Building from source

Requirements: Go (see `go.mod`) with cgo (a C compiler, for the embedded DuckDB), Node.js 22+,
`make`.

```sh
make test      # run all tests
make build     # build ./bin/obsrv with the embedded UI
make run       # build and run with data in ./data
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Design principles](docs/DESIGN.md)
- [Architecture Decision Records](docs/adr/)

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md) first.
To report a security vulnerability, follow [SECURITY.md](SECURITY.md) and do not open a public issue.

## License

obsrv is licensed under the [Apache License 2.0](LICENSE).
