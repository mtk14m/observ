// Package layout defines where telemetry files live in the object store.
// The layout is part of the public storage contract:
//
//	v1/<signal>/date=YYYY-MM-DD/hour=HH/<ulid>.parquet
//
// Hours are UTC. A file only holds rows whose time falls in its hour, so
// readers can prune files by partition.
package layout

import (
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/mtk14n/obsrv/pkg/schema"
)

// Signal directories.
const (
	Logs         = "logs"
	Spans        = "spans"
	MetricPoints = "metric_points"
)

// Dirs lists every signal directory.
var Dirs = []string{Logs, Spans, MetricPoints}

const partitionFormat = "date=2006-01-02/hour=15"

// Prefix returns the key prefix of a signal directory.
func Prefix(dir string) string { return schema.Version + "/" + dir + "/" }

// Partition returns the partition path for t.
func Partition(t time.Time) string { return t.UTC().Format(partitionFormat) }

// NewKey returns a new, unique key for a file of dir holding rows of t's hour.
func NewKey(dir string, t time.Time) string {
	return Prefix(dir) + Partition(t) + "/" + ulid.Make().String() + ".parquet"
}

// Key is a parsed file key.
type Key struct {
	Dir string
	// Hour is the start of the partition's hour, in UTC.
	Hour time.Time
	// Partition is the key prefix shared by the files of the same hour.
	Partition string
}

// ParseKey parses a data file key. It reports false for any other key.
func ParseKey(key string) (Key, bool) {
	rest, ok := strings.CutPrefix(key, schema.Version+"/")
	if !ok {
		return Key{}, false
	}
	dir, rest, ok := strings.Cut(rest, "/")
	if !ok || !strings.HasSuffix(rest, ".parquet") || len(rest) < len(partitionFormat)+1 {
		return Key{}, false
	}
	hour, err := time.Parse(partitionFormat, rest[:len(partitionFormat)])
	if err != nil || rest[len(partitionFormat)] != '/' {
		return Key{}, false
	}
	return Key{Dir: dir, Hour: hour, Partition: Prefix(dir) + rest[:len(partitionFormat)+1]}, true
}
