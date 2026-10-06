// Command obsrv runs the obsrv observability engine.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/otlp"
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
	fs := flag.NewFlagSet("obsrv", flag.ContinueOnError)
	fs.SetOutput(out)
	var (
		showVersion  = fs.Bool("version", false, "print version and exit")
		otlpGRPCAddr = fs.String("otlp-grpc-addr", ":4317", "listen address for OTLP/gRPC")
		otlpHTTPAddr = fs.String("otlp-http-addr", ":4318", "listen address for OTLP/HTTP")
		httpAddr     = fs.String("http-addr", ":8080", "listen address for the API and UI")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(out, version.String())
		return err
	}

	log := slog.New(slog.NewTextHandler(out, nil))
	log.Info("starting", "version", version.Version, "commit", version.Commit)

	sink := &ingest.Counter{}
	g, ctx := errgroup.WithContext(ctx)
	grpcSrv := grpc.NewServer(grpc.MaxRecvMsgSize(otlp.DefaultMaxBodyBytes))
	otlp.RegisterGRPC(grpcSrv, sink)
	serveGRPC(ctx, g, log, *otlpGRPCAddr, grpcSrv)
	serve(ctx, g, log, "otlp-http", *otlpHTTPAddr, otlp.NewHTTPHandler(sink, otlp.HTTPOptions{}))
	serve(ctx, g, log, "api", *httpAddr, newAPIHandler())
	g.Go(func() error { reportStats(ctx, log, sink); return nil })
	return g.Wait()
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

func newAPIHandler() http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") }
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", ok)
	return mux
}

// reportStats logs ingestion totals every 10 seconds while they change.
func reportStats(ctx context.Context, log *slog.Logger, c *ingest.Counter) {
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
