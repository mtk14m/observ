package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mtk14n/obsrv/internal/objstore/s3"
)

// config is obsrv's configuration. Every flag can also be set with an
// environment variable: -s3-bucket is OBSRV_S3_BUCKET. Flags win.
type config struct {
	showVersion   bool
	otlpGRPCAddr  string
	otlpHTTPAddr  string
	httpAddr      string
	dataDir       string
	retention     time.Duration
	storage       string
	s3            s3.Config
	cacheSizeMB   int
	publicURL     string
	alertInterval time.Duration
	ingestToken   string
	adminEmail    string
	adminPassword string
}

// secureCookies reports whether obsrv is served over HTTPS, in which case
// the session cookie must only travel over HTTPS.
func (c config) secureCookies() bool { return strings.HasPrefix(c.publicURL, "https://") }

func parseConfig(args []string, getenv func(string) string, out io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet("obsrv", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.BoolVar(&c.showVersion, "version", false, "print version and exit")
	fs.StringVar(&c.otlpGRPCAddr, "otlp-grpc-addr", ":4317", "listen address for OTLP/gRPC")
	fs.StringVar(&c.otlpHTTPAddr, "otlp-http-addr", ":4318", "listen address for OTLP/HTTP")
	fs.StringVar(&c.httpAddr, "http-addr", ":8080", "listen address for the API and UI")
	fs.StringVar(&c.dataDir, "data-dir", "./data", "directory for the WAL, local storage and the query cache")
	fs.DurationVar(&c.retention, "retention", 7*24*time.Hour, "how long to keep telemetry (0 keeps it forever)")
	fs.StringVar(&c.storage, "storage", "fs", "where telemetry is stored: fs (data-dir) or s3")
	fs.StringVar(&c.s3.Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 endpoint (host[:port])")
	fs.StringVar(&c.s3.Bucket, "s3-bucket", "", "S3 bucket (required with -storage=s3)")
	fs.StringVar(&c.s3.Prefix, "s3-prefix", "", "optional key prefix inside the bucket")
	fs.StringVar(&c.s3.Region, "s3-region", "", "S3 region")
	fs.BoolVar(&c.s3.Insecure, "s3-insecure", false, "use plain HTTP to reach S3 (local MinIO)")
	fs.IntVar(&c.cacheSizeMB, "cache-size-mb", 2048, "size of the local query cache with -storage=s3")
	fs.StringVar(&c.publicURL, "public-url", "http://localhost:8080", "URL where users reach obsrv, used in alert notifications")
	fs.DurationVar(&c.alertInterval, "alert-interval", 30*time.Second, "how often alert rules are evaluated")
	fs.StringVar(&c.ingestToken, "ingest-token", "", "require OTLP clients to send 'Authorization: Bearer <token>'")
	fs.StringVar(&c.adminEmail, "admin-email", "", "create this admin account on first start (otherwise the UI asks)")
	fs.StringVar(&c.adminPassword, "admin-password", "", "password of -admin-email")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(out, "Usage: obsrv [flags]\n\nEvery flag can be set with an environment variable, e.g. -s3-bucket as OBSRV_S3_BUCKET.\nS3 credentials come from AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, ~/.aws or an IAM role.\n\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return c, err
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var errs []error
	fs.VisitAll(func(f *flag.Flag) {
		if set[f.Name] {
			return
		}
		name := "OBSRV_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v := getenv(name); v != "" {
			if err := f.Value.Set(v); err != nil {
				errs = append(errs, fmt.Errorf("%s=%q: %w", name, v, err))
			}
		}
	})
	if err := errors.Join(errs...); err != nil {
		return c, err
	}

	switch c.storage {
	case "fs":
	case "s3":
		if c.s3.Bucket == "" {
			return c, errors.New("-storage=s3 requires -s3-bucket")
		}
	default:
		return c, fmt.Errorf("unknown -storage %q: use fs or s3", c.storage)
	}
	if (c.adminEmail == "") != (c.adminPassword == "") {
		return c, errors.New("-admin-email and -admin-password must be set together")
	}
	if c.alertInterval < 5*time.Second {
		return c, errors.New("-alert-interval must be at least 5s")
	}
	if c.cacheSizeMB < 0 {
		return c, errors.New("-cache-size-mb must be positive")
	}
	return c, nil
}
