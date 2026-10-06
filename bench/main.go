// Command bench measures obsrv on generated, realistic telemetry: ingestion
// throughput and cost, storage size and compression, compaction, and query
// latency. Run it with `make bench`; results are printed as Markdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/compact"
	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/query"
)

type batch struct {
	kind  string
	proto []byte
	items int
}

func main() {
	nLogs := flag.Int("logs", 500_000, "log records to ingest")
	nSpans := flag.Int("spans", 300_000, "spans to ingest")
	ticks := flag.Int("metric-ticks", 360, "metric reports per series (every 10s)")
	nServices := flag.Int("services", 20, "services in the generated fleet")
	batchSize := flag.Int("batch", 1000, "items per OTLP request")
	flushEvery := flag.Int("flush-every", 50_000, "flush after this many items, like a 10s flush under load")
	flag.Parse()

	dir, err := os.MkdirTemp("", "obsrv-bench-*")
	check(err)
	if os.Getenv("BENCH_KEEP") == "" {
		defer func() { _ = os.RemoveAll(dir) }()
	} else {
		fmt.Fprintln(os.Stderr, "keeping", dir)
	}
	ctx := context.Background()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Generate every request up front so that only server work is measured.
	g := newGenerator(42, *nServices, start)
	var batches []batch
	for off := 0; off < *nLogs; off += *batchSize {
		b, _ := (&plog.ProtoMarshaler{}).MarshalLogs(g.logs(min(*batchSize, *nLogs-off), off, *nLogs, 59*time.Minute))
		batches = append(batches, batch{"logs", b, min(*batchSize, *nLogs-off)})
	}
	for off := 0; off < *nSpans; off += *batchSize {
		td := g.traces(min(*batchSize, *nSpans-off), off, *nSpans, 59*time.Minute)
		b, _ := (&ptrace.ProtoMarshaler{}).MarshalTraces(td)
		batches = append(batches, batch{"traces", b, td.SpanCount()})
	}
	for tick := range *ticks {
		md := g.metrics(tick, start.Add(time.Duration(tick)*10*time.Second))
		b, _ := (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
		batches = append(batches, batch{"metrics", b, md.DataPointCount()})
	}
	otlpBytes := map[string]int{}
	items := map[string]int{}
	for _, b := range batches {
		otlpBytes[b.kind] += len(b.proto)
		items[b.kind] += b.items
	}

	store, err := fs.New(filepath.Join(dir, "store"))
	check(err)
	p, err := ingest.New(ingest.Options{WALDir: filepath.Join(dir, "wal"), Store: store, MaxBufferedRows: 10_000_000})
	check(err)

	// --- Ingestion -------------------------------------------------------------
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	cpu0 := cpuTime()
	t0 := time.Now()
	sinceFlush := 0
	for _, b := range batches {
		switch b.kind {
		case "logs":
			ld, err := (&plog.ProtoUnmarshaler{}).UnmarshalLogs(b.proto)
			check(err)
			check(p.ConsumeLogs(ctx, ld))
		case "traces":
			td, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(b.proto)
			check(err)
			check(p.ConsumeTraces(ctx, td))
		case "metrics":
			md, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(b.proto)
			check(err)
			check(p.ConsumeMetrics(ctx, md))
		}
		if sinceFlush += b.items; sinceFlush >= *flushEvery {
			check(p.Flush(ctx))
			sinceFlush = 0
		}
	}
	check(p.Flush(ctx))
	wall := time.Since(t0)
	cpu := cpuTime() - cpu0
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	check(p.Close())

	flushed := sizes(ctx, store)

	// --- Compaction ------------------------------------------------------------
	c := compact.New(compact.Options{Store: store, Now: func() time.Time { return start.Add(48 * time.Hour) }})
	c0 := time.Now()
	res, err := c.RunOnce(ctx)
	check(err)
	compactTime := time.Since(c0)
	compacted := sizes(ctx, store)

	// --- Queries ---------------------------------------------------------------
	e, err := query.New(store)
	check(err)
	r := query.TimeRange{From: start, To: start.Add(time.Hour)}
	traces, err := e.SearchTraces(ctx, query.TraceQuery{TimeRange: r, Limit: 1})
	check(err)
	queries := []struct {
		name string
		run  func() error
	}{
		{"Services (RED per service)", func() error { _, err := e.Services(ctx, r); return err }},
		{"Logs: `level:error`, 100 newest", func() error {
			_, err := e.SearchLogs(ctx, query.LogQuery{TimeRange: r, Query: "level:error"})
			return err
		}},
		{"Logs: full-text `declined`", func() error {
			_, err := e.SearchLogs(ctx, query.LogQuery{TimeRange: r, Query: "declined"})
			return err
		}},
		{"Log volume histogram", func() error { _, err := e.LogHistogram(ctx, query.LogQuery{TimeRange: r}, 0); return err }},
		{"Trace search, 50 newest", func() error { _, err := e.SearchTraces(ctx, query.TraceQuery{TimeRange: r}); return err }},
		{"Trace by ID", func() error { _, err := e.Trace(ctx, traces[0].TraceID, r); return err }},
		{"Metric: p95 latency by service", func() error {
			_, err := e.QueryMetric(ctx, query.MetricQuery{TimeRange: r, Metric: "http.server.request.duration", Agg: "p95", GroupBy: []string{"service.name"}})
			return err
		}},
		{"Metric: request rate by route", func() error {
			_, err := e.QueryMetric(ctx, query.MetricQuery{TimeRange: r, Metric: "http.server.requests", GroupBy: []string{"http.route"}})
			return err
		}},
	}

	// --- Report ----------------------------------------------------------------
	total := 0
	totalBytes := 0
	for k := range items {
		total += items[k]
		totalBytes += otlpBytes[k]
	}
	fmt.Printf("## Run of %s\n\n", time.Now().Format("2006-01-02"))
	fmt.Printf("Machine: %s/%s, %d CPUs, %s. Dataset: %d services over one hour, batches of %d, flush every %d items.\n\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), *nServices, *batchSize, *flushEvery)

	fmt.Println("### Ingestion (decode + convert + WAL fsync + buffer + Parquet write)")
	fmt.Println()
	fmt.Println("| Items | OTLP bytes | Wall time | Items/s | OTLP MB/s | CPU time | CPU-seconds per GB of OTLP | GC cycles | Peak RSS |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|")
	fmt.Printf("| %s | %s | %s | %s | %.1f | %s | %.1f | %d | %s |\n\n",
		human(float64(total)), bytesH(totalBytes), wall.Round(time.Millisecond), human(float64(total)/wall.Seconds()),
		float64(totalBytes)/1e6/wall.Seconds(), cpu.Round(time.Millisecond), cpu.Seconds()/(float64(totalBytes)/1e9),
		after.NumGC-before.NumGC, bytesH(maxRSS()))

	fmt.Println("### Storage")
	fmt.Println()
	fmt.Println("| Signal | Items | OTLP protobuf | Parquet (flushed) | Files | Parquet (compacted) | Files | Ratio vs OTLP | Bytes per item |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|")
	for _, s := range []struct{ kind, dir string }{{"logs", layout.Logs}, {"traces", layout.Spans}, {"metrics", layout.MetricPoints}} {
		f, cpt := flushed[s.dir], compacted[s.dir]
		fmt.Printf("| %s | %s | %s | %s | %d | %s | %d | **%.1f×** | %.1f |\n", s.kind, human(float64(items[s.kind])),
			bytesH(otlpBytes[s.kind]), bytesH(f.bytes), f.files, bytesH(cpt.bytes), cpt.files,
			float64(otlpBytes[s.kind])/float64(cpt.bytes), float64(cpt.bytes)/float64(items[s.kind]))
	}
	fmt.Printf("\nCompaction merged %d files into %d in %s.\n\n", res.FilesMerged, res.FilesWritten, compactTime.Round(time.Millisecond))

	fmt.Println("### Queries (one hour of data, median of 5 runs)")
	fmt.Println()
	fmt.Println("| Query | Latency |")
	fmt.Println("|---|---|")
	for _, q := range queries {
		var d []time.Duration
		for range 5 {
			t := time.Now()
			check(q.run())
			d = append(d, time.Since(t))
		}
		slices.Sort(d)
		fmt.Printf("| %s | %s |\n", q.name, d[2].Round(100*time.Microsecond))
	}
}

type dirSize struct {
	bytes int
	files int
}

func sizes(ctx context.Context, store *fs.Store) map[string]dirSize {
	out := map[string]dirSize{}
	for _, d := range layout.Dirs {
		infos, err := store.List(ctx, layout.Prefix(d))
		check(err)
		var s dirSize
		for _, i := range infos {
			s.bytes += int(i.Size)
			s.files++
		}
		out[d] = s
	}
	return out
}

func cpuTime() time.Duration {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func maxRSS() int {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	if runtime.GOOS == "darwin" {
		return int(ru.Maxrss) // bytes
	}
	return int(ru.Maxrss) * 1024 // kilobytes on Linux
}

func human(v float64) string {
	switch {
	case v >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fK", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

func bytesH(b int) string {
	units := []string{"B", "KB", "MB", "GB"}
	v := float64(b)
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0") + " " + units[i]
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}
