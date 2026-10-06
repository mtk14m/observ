// Package query answers questions about stored telemetry: services, logs,
// traces and metrics. It prunes the Parquet files to read using their
// partitions and lets an embedded DuckDB scan them.
package query
