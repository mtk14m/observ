# Benchmark results

Produced by `make bench` (`go run ./bench`). The generator builds realistic telemetry for a fleet
of services running in Kubernetes: log lines from templates with variable parts (IDs, IPs,
durations), traces crossing 3 to 6 services, and gauge, counter and histogram series reported
every 10 seconds. Requests are generated before the measurement starts, so the numbers only cover
server work: OTLP decoding, conversion, WAL with fsync, buffering and Parquet writing.

"Ratio vs OTLP" compares stored Parquet bytes with **uncompressed OTLP protobuf**, which is
already a compact binary format. Ratios against JSON logs or against row-oriented indexes would
be much higher.

## Run of 2026-10-07

Machine: darwin/arm64, 10 CPUs, go1.27.1. Dataset: 20 services over one hour, batches of 1000, flush every 50000 items.

### Ingestion (decode + convert + WAL fsync + buffer + Parquet write)

| Items | OTLP bytes | Wall time | Items/s | OTLP MB/s | CPU time | CPU-seconds per GB of OTLP | GC cycles | Peak RSS |
|---|---|---|---|---|---|---|---|---|
| 908.5K | 230.3 MB | 7.421s | 122.4K | 31.0 | 3.292s | 14.3 | 15 | 1.4 GB |

### Storage

| Signal | Items | OTLP protobuf | Parquet (flushed) | Files | Parquet (compacted) | Files | Ratio vs OTLP | Bytes per item |
|---|---|---|---|---|---|---|---|---|
| logs | 500.0K | 66.4 MB | 14.5 MB | 10 | 14.5 MB | 1 | **4.6×** | 29.1 |
| traces | 300.5K | 148.2 MB | 14.4 MB | 6 | 14.8 MB | 1 | **10.0×** | 49.3 |
| metrics | 108.0K | 15.7 MB | 372.7 KB | 3 | 392.1 KB | 1 | **40.1×** | 3.6 |

Compaction merged 19 files into 3 in 3.27s.

### Queries (one hour of data, median of 5 runs)

| Query | Latency |
|---|---|
| Services (RED per service) | 6.5ms |
| Logs: `level:error`, 100 newest | 40.2ms |
| Logs: full-text `declined` | 57.5ms |
| Log volume histogram | 3.1ms |
| Trace search, 50 newest | 21.3ms |
| Trace by ID | 16ms |
| Metric: p95 latency by service | 94.6ms |
| Metric: request rate by route | 45.5ms |

### Reading these numbers

- **Ingestion is not the bottleneck.** About 14 CPU-seconds per GB of OTLP means one core can
  ingest several TB per day. The product target of 50 GB/day on 4 vCPU (0.6 MB/s) is exceeded by
  a wide margin. The peak RSS includes the pre-generated requests (about 230 MB) and the
  generator, so it does not reflect the server's own footprint.
- **Metrics (40×) and traces (10×)** meet the compression targets.
- **Logs (about 4.6×) miss the 10× target.** A per-column analysis
  (`SELECT path_in_schema, sum(total_compressed_size) FROM parquet_metadata(...)`) shows that
  most of the remaining bytes are high-entropy data: random trace IDs stored as hex strings,
  random numbers in log bodies, and user IDs. The generator is pessimistic here: in real
  workloads many logs share a trace ID and bodies repeat more. The next levers are:
  1. storing trace and span IDs as 16- and 8-byte binary values instead of hex strings
     (about −30% on that column, but it is a breaking schema change, to decide before v1);
  2. moving resource attributes to a deduplicated `resources` table, as planned in the
     architecture;
  3. measuring on real logs (the OpenTelemetry demo) before optimising further.
- **Queries** stay under 100 ms on one hour of data (about 900K items) on a laptop.

### History

| Date | Change | Logs | Traces | Metrics |
|---|---|---|---|---|
| 2026-10-07 | First run | 4.0× | 9.6× | 36.7× |
| 2026-10-07 | Delta encoding for timestamps, dictionaries for map keys and resource values | 4.6× | 10.0× | 40.1× |
