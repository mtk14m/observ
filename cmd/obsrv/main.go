// Command obsrv runs the obsrv observability engine.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/mtk14n/obsrv/internal/api"
	"github.com/mtk14n/obsrv/internal/compact"
	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore"
	"github.com/mtk14n/obsrv/internal/objstore/cache"
	objfs "github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/objstore/s3"
	"github.com/mtk14n/obsrv/internal/otlp"
	"github.com/mtk14n/obsrv/internal/query"
	"github.com/mtk14n/obsrv/internal/ui"
	"github.com/mtk14n/obsrv/internal/version"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "obsrv:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	cfg, err := parseConfig(args, os.Getenv, out)
	if err != nil {
		return err
	}
	if cfg.showVersion {
		_, err := fmt.Fprintln(out, version.String())
		return err
	}

	log := slog.New(slog.NewTextHandler(out, nil))
	log.Info("starting", "version", version.Version, "commit", version.Commit, "storage", cfg.storage)

	store, files, err := openStorage(ctx, cfg)
	if err != nil {
		return err
	}
	sink, err := ingest.New(ingest.Options{
		WALDir: filepath.Join(cfg.dataDir, "wal"),
		Store:  store,
		Logger: log,
	})
	if err != nil {
		return err
	}
	defer func() { _ = sink.Close() }()

	engine, err := query.New(files, query.WithHot(sink))
	if err != nil {
		return err
	}
	defer func() { _ = engine.Close() }()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return sink.Run(ctx) })
	compactor := compact.New(compact.Options{Store: store, Retention: cfg.retention, Logger: log})
	g.Go(func() error { compactor.Run(ctx, time.Minute); return nil })
	grpcSrv := grpc.NewServer(grpc.MaxRecvMsgSize(otlp.DefaultMaxBodyBytes))
	otlp.RegisterGRPC(grpcSrv, sink)
	serveGRPC(ctx, g, log, cfg.otlpGRPCAddr, grpcSrv)
	serve(ctx, g, log, "otlp-http", cfg.otlpHTTPAddr, otlp.NewHTTPHandler(sink, otlp.HTTPOptions{}))
	serve(ctx, g, log, "http", cfg.httpAddr, newAPIHandler(engine))
	g.Go(func() error { reportStats(ctx, log, sink); return nil })
	return g.Wait()
}

// openStorage returns the object store telemetry is written to, and the
// file source the query engine reads from: the local store itself, or a
// disk cache in front of S3.
func openStorage(ctx context.Context, cfg config) (objstore.ObjectStore, query.FileSource, error) {
	if cfg.storage == "s3" {
		store, err := s3.New(ctx, cfg.s3)
		if err != nil {
			return nil, nil, err
		}
		files, err := cache.New(cache.Options{
			Store:    store,
			Dir:      filepath.Join(cfg.dataDir, "cache"),
			MaxBytes: int64(cfg.cacheSizeMB) << 20,
		})
		if err != nil {
			return nil, nil, err
		}
		return store, files, nil
	}
	store, err := objfs.New(filepath.Join(cfg.dataDir, "store"))
	if err != nil {
		return nil, nil, err
	}
	return store, store, nil
}

// serve runs an HTTP server until ctx is done, then shuts it down gracefully.
func serve(ctx context.Context, g *errgroup.Group, log *slog.Logger, name, addr string, h http.Handler) {
	g.Go(func() error {
		ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("%s: listen: %w", name, err)
		}
		srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
		log.Info("listening", "server", name, "addr", ln.Addr().String())

		errc := make(chan error, 1)
		go func() { errc <- srv.Serve(ln) }()

		select {
		case err := <-errc:
			return fmt.Errorf("%s: %w", name, err)
		case <-ctx.Done():
		}
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("%s: shutdown: %w", name, err)
		}
		if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	})
}

// serveGRPC runs the gRPC server until ctx is done, then stops it gracefully.
func serveGRPC(ctx context.Context, g *errgroup.Group, log *slog.Logger, addr string, srv *grpc.Server) {
	g.Go(func() error {
		ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("otlp-grpc: listen: %w", err)
		}
		log.Info("listening", "server", "otlp-grpc", "addr", ln.Addr().String())
		errc := make(chan error, 1)
		go func() { errc <- srv.Serve(ln) }()
		select {
		case err := <-errc:
			return fmt.Errorf("otlp-grpc: %w", err)
		case <-ctx.Done():
			srv.GracefulStop()
			return nil
		}
	})
}

// newAPIHandler serves health checks, the JSON API and the embedded UI.
func newAPIHandler(q api.Querier) http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") }
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", ok)
	mux.Handle("/api/", api.NewHandler(q, api.Options{}))
	mux.Handle("/", ui.NewHandler(ui.Dist()))
	return mux
}

// reportStats logs ingestion totals every 10 seconds while they change.
func reportStats(ctx context.Context, log *slog.Logger, c *ingest.Pipeline) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	var last ingest.Stats
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s := c.Stats(); s != last {
				log.Info("ingested", "spans", s.Spans, "data_points", s.DataPoints, "log_records", s.LogRecords)
				last = s
			}
		}
	}
}
