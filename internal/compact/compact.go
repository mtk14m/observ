// Package compact merges the small files written by ingestion and enforces
// retention.
//
// Ingestion writes a few files per signal every flush. Once an hour is
// closed (plus a grace period for late data), the compactor merges its
// files into files of about TargetBytes, re-sorted for compression.
//
// A merge is crash-safe: a manifest naming the output and its sources is
// written first, then the output, then the sources are deleted, then the
// manifest. On start, leftover manifests are resolved: if the output exists
// the sources are deleted, otherwise the manifest is dropped and the
// sources stay. Readers listing files during the few milliseconds between
// writing the output and deleting the sources can see rows twice; a
// metadata catalog will make swaps atomic later.
package compact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/internal/layout"
	"github.com/mtk14n/obsrv/internal/model"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/pkg/schema"
)

const (
	manifestPrefix     = schema.Version + "/_compaction/"
	defaultGrace       = 5 * time.Minute
	defaultTargetBytes = 128 << 20
)

// Options configures a Compactor.
type Options struct {
	Store objstore.ObjectStore
	// Retention deletes data older than this. Zero keeps data forever.
	Retention time.Duration
	// Grace is how long after the end of an hour late data may still
	// arrive before the hour is compacted. Defaults to 5 minutes.
	Grace time.Duration
	// TargetBytes bounds the size of the input of each merge. Defaults to 128 MiB.
	TargetBytes int64
	Now         func() time.Time
	Logger      *slog.Logger
}

// Result summarises a compaction run.
type Result struct {
	FilesMerged  int
	FilesWritten int
	FilesExpired int
}

// Compactor merges small files and applies retention.
type Compactor struct {
	opts Options
}

// New returns a Compactor.
func New(opts Options) *Compactor {
	if opts.Grace <= 0 {
		opts.Grace = defaultGrace
	}
	if opts.TargetBytes <= 0 {
		opts.TargetBytes = defaultTargetBytes
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return &Compactor{opts: opts}
}

// Run compacts every interval until ctx is done.
func (c *Compactor) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		res, err := c.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			c.opts.Logger.Error("compaction failed, will retry", "err", err)
		} else if res != (Result{}) {
			c.opts.Logger.Info("compacted", "merged", res.FilesMerged, "written", res.FilesWritten, "expired", res.FilesExpired)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunOnce resolves interrupted merges, applies retention and merges every
// closed hour that has more than one file.
func (c *Compactor) RunOnce(ctx context.Context) (Result, error) {
	var res Result
	if err := c.recover(ctx); err != nil {
		return res, err
	}
	now := c.opts.Now()
	for _, dir := range layout.Dirs {
		infos, err := c.opts.Store.List(ctx, layout.Prefix(dir))
		if err != nil {
			return res, fmt.Errorf("compact: list %s: %w", dir, err)
		}
		partitions := map[string][]objstore.ObjectInfo{}
		hours := map[string]time.Time{}
		var order []string
		for _, info := range infos {
			k, ok := layout.ParseKey(info.Key)
			if !ok {
				continue
			}
			if _, seen := partitions[k.Partition]; !seen {
				order = append(order, k.Partition)
				hours[k.Partition] = k.Hour
			}
			partitions[k.Partition] = append(partitions[k.Partition], info)
		}

		for _, part := range order {
			files, end := partitions[part], hours[part].Add(time.Hour)
			if c.opts.Retention > 0 && !end.After(now.Add(-c.opts.Retention)) {
				for _, f := range files {
					if err := c.opts.Store.Delete(ctx, f.Key); err != nil {
						return res, fmt.Errorf("compact: expire %s: %w", f.Key, err)
					}
					res.FilesExpired++
				}
				continue
			}
			if end.Add(c.opts.Grace).After(now) {
				continue // the hour may still receive data
			}
			for _, group := range c.groups(files) {
				if err := c.merge(ctx, dir, hours[part], group); err != nil {
					return res, err
				}
				res.FilesMerged += len(group)
				res.FilesWritten++
			}
		}
	}
	return res, nil
}

// groups splits files into consecutive groups of at most TargetBytes and
// keeps only the groups with something to merge.
func (c *Compactor) groups(files []objstore.ObjectInfo) [][]string {
	var out [][]string
	var cur []string
	var size int64
	flush := func() {
		if len(cur) > 1 {
			out = append(out, cur)
		}
		cur, size = nil, 0
	}
	for _, f := range files {
		if len(cur) > 0 && size+f.Size > c.opts.TargetBytes {
			flush()
		}
		cur = append(cur, f.Key)
		size += f.Size
	}
	flush()
	return out
}

type manifest struct {
	Output  string   `json:"output"`
	Sources []string `json:"sources"`
}

func manifestKey(output string) string {
	return manifestPrefix + strings.TrimSuffix(path.Base(output), ".parquet") + ".json"
}

func (c *Compactor) merge(ctx context.Context, dir string, hour time.Time, sources []string) error {
	var (
		data []byte
		err  error
	)
	switch dir {
	case layout.Logs:
		data, err = mergeRows(ctx, c.opts.Store, sources, model.SortLogs)
	case layout.Spans:
		data, err = mergeRows(ctx, c.opts.Store, sources, model.SortSpans)
	case layout.MetricPoints:
		data, err = mergeRows(ctx, c.opts.Store, sources, model.SortMetricPoints)
	default:
		return fmt.Errorf("compact: unknown signal directory %q", dir)
	}
	if err != nil {
		return err
	}

	output := layout.NewKey(dir, hour)
	m, _ := json.Marshal(manifest{Output: output, Sources: sources})
	mkey := manifestKey(output)
	if err := c.opts.Store.Put(ctx, mkey, bytes.NewReader(m)); err != nil {
		return fmt.Errorf("compact: write manifest: %w", err)
	}
	if err := c.opts.Store.Put(ctx, output, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("compact: write %s: %w", output, err)
	}
	return c.finish(ctx, mkey, sources)
}

// finish deletes the sources of a completed merge, then its manifest.
func (c *Compactor) finish(ctx context.Context, mkey string, sources []string) error {
	for _, s := range sources {
		if err := c.opts.Store.Delete(ctx, s); err != nil {
			return fmt.Errorf("compact: delete %s: %w", s, err)
		}
	}
	if err := c.opts.Store.Delete(ctx, mkey); err != nil {
		return fmt.Errorf("compact: delete manifest: %w", err)
	}
	return nil
}

// recover resolves merges interrupted by a crash.
func (c *Compactor) recover(ctx context.Context) error {
	infos, err := c.opts.Store.List(ctx, manifestPrefix)
	if err != nil {
		return fmt.Errorf("compact: list manifests: %w", err)
	}
	for _, info := range infos {
		b, err := readAll(ctx, c.opts.Store, info.Key)
		if err != nil {
			return err
		}
		var m manifest
		if err := json.Unmarshal(b, &m); err != nil || m.Output == "" {
			c.opts.Logger.Warn("dropping unreadable compaction manifest", "key", info.Key)
			if err := c.opts.Store.Delete(ctx, info.Key); err != nil {
				return err
			}
			continue
		}
		rc, err := c.opts.Store.Get(ctx, m.Output)
		switch {
		case err == nil:
			_ = rc.Close()
			if err := c.finish(ctx, info.Key, m.Sources); err != nil {
				return err
			}
		case errors.Is(err, objstore.ErrNotFound):
			if err := c.opts.Store.Delete(ctx, info.Key); err != nil {
				return err
			}
		default:
			return fmt.Errorf("compact: check %s: %w", m.Output, err)
		}
	}
	return nil
}

func mergeRows[T any](ctx context.Context, store objstore.ObjectStore, keys []string, sortRows func([]T)) ([]byte, error) {
	var rows []T
	for _, k := range keys {
		b, err := readAll(ctx, store, k)
		if err != nil {
			return nil, err
		}
		got, err := parquet.Read[T](bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return nil, fmt.Errorf("compact: read %s: %w", k, err)
		}
		rows = append(rows, got...)
	}
	sortRows(rows)
	var buf bytes.Buffer
	if err := parquet.Write(&buf, rows); err != nil {
		return nil, fmt.Errorf("compact: encode: %w", err)
	}
	return buf.Bytes(), nil
}

func readAll(ctx context.Context, store objstore.ObjectStore, key string) ([]byte, error) {
	rc, err := store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("compact: read %s: %w", key, err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("compact: read %s: %w", key, err)
	}
	return b, nil
}
