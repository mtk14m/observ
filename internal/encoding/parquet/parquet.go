// Package parquet writes and reads rows of the public schema as Parquet
// files compressed with ZSTD.
package parquet

import (
	"errors"
	"fmt"
	"io"

	pq "github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/zstd"
)

// rowGroupRows bounds row groups so readers can prune and parallelise.
const rowGroupRows = 128 * 1024

// Write encodes rows as one Parquet file into w.
func Write[T any](w io.Writer, rows []T) error {
	pw := pq.NewGenericWriter[T](w,
		pq.Compression(&zstd.Codec{Level: zstd.SpeedDefault}),
		pq.MaxRowsPerRowGroup(rowGroupRows),
	)
	if _, err := pw.Write(rows); err != nil {
		return fmt.Errorf("parquet: write rows: %w", err)
	}
	if err := pw.Close(); err != nil {
		return fmt.Errorf("parquet: close: %w", err)
	}
	return nil
}

// Read decodes every row of a Parquet file.
func Read[T any](r io.ReaderAt, size int64) ([]T, error) {
	f, err := pq.OpenFile(r, size)
	if err != nil {
		return nil, fmt.Errorf("parquet: open: %w", err)
	}
	pr := pq.NewGenericReader[T](f)
	defer func() { _ = pr.Close() }()

	rows := make([]T, f.NumRows())
	n, err := pr.Read(rows)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parquet: read rows: %w", err)
	}
	return rows[:n], nil
}
