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

## Quick start

The quick start below describes the target experience. It is not available yet.

```sh
docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 -v obsrv:/data ghcr.io/mtk14n/obsrv
```

Point any OpenTelemetry SDK or Collector at `http://localhost:4318`, then open <http://localhost:8080>.

## Building from source

Requirements: Go (see `go.mod`), Node.js 22+, `make`.

```sh
make test      # run all tests
make build     # build ./bin/obsrv
./bin/obsrv -version
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
