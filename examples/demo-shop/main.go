// Command demo-shop is a small multi-service shop used to demo obsrv.
//
// The same binary runs one service, chosen with -role (or DEMO_ROLE):
//
//	frontend   serves /products and /checkout and generates load on itself
//	checkout   places orders: reserves stock, then charges the payment
//	inventory  serves stock levels and reservations
//	payment    charges cards; one issuer (acme-bank) is often slow and
//	           refuses about a quarter of its cards
//
// It is instrumented only with the official OpenTelemetry Go SDK and
// configured with the standard OTEL_* environment variables.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("demo-shop")
	meter  = otel.Meter("demo-shop")
	log    *slog.Logger
	client = &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 5 * time.Second}
)

var products = []string{"espresso", "croissant", "baguette", "macaron", "eclair"}

func main() {
	role := flag.String("role", os.Getenv("DEMO_ROLE"), "frontend, checkout, inventory or payment")
	addr := flag.String("addr", envOr("DEMO_ADDR", ":8000"), "listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := setupTelemetry(ctx, *role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "telemetry:", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()
	log = otelslog.NewLogger("demo-shop")

	mux := http.NewServeMux()
	switch *role {
	case "frontend":
		mux.HandleFunc("GET /products", frontendProducts)
		mux.HandleFunc("POST /checkout", frontendCheckout)
		go generateLoad(ctx, selfURL(*addr))
	case "checkout":
		mux.HandleFunc("POST /checkout", checkout)
	case "inventory":
		registerStockGauge()
		mux.HandleFunc("GET /stock", stock)
		mux.HandleFunc("POST /reserve", reserve)
	case "payment":
		mux.HandleFunc("POST /pay", pay)
	default:
		fmt.Fprintln(os.Stderr, "unknown role", *role)
		os.Exit(2)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           otelhttp.NewHandler(mux, *role, otelhttp.WithSpanNameFormatter(spanName)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	slog.Info("listening", "role", *role, "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func spanName(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path }

// --- frontend ---------------------------------------------------------------

func frontendProducts(w http.ResponseWriter, r *http.Request) {
	if err := call(r.Context(), http.MethodGet, envOr("INVENTORY_URL", "http://inventory:8000")+"/stock"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func frontendCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	product := r.URL.Query().Get("product")
	log.InfoContext(ctx, "checkout started", "product", product)
	if err := call(ctx, http.MethodPost, envOr("CHECKOUT_URL", "http://checkout:8000")+"/checkout?product="+product); err != nil {
		log.WarnContext(ctx, "checkout failed, showing an error page", "product", product, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func generateLoad(ctx context.Context, base string) {
	time.Sleep(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(150+rand.IntN(250)) * time.Millisecond):
		}
		product := products[rand.IntN(len(products))]
		path := "/products"
		method := http.MethodGet
		if rand.IntN(3) == 0 {
			path, method = "/checkout?product="+product, http.MethodPost
		}
		req, _ := http.NewRequestWithContext(ctx, method, base+path, nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// --- checkout ---------------------------------------------------------------

var orders, _ = meter.Int64Counter("shop.orders", metric.WithDescription("Orders by outcome"), metric.WithUnit("{order}"))

func checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	product := r.URL.Query().Get("product")
	orderID := strconv.Itoa(100000 + rand.IntN(900000))
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("order.id", orderID), attribute.String("product", product))

	if err := call(ctx, http.MethodPost, envOr("INVENTORY_URL", "http://inventory:8000")+"/reserve?product="+product); err != nil {
		fail(ctx, w, "reservation failed", orderID, err)
		return
	}
	if err := call(ctx, http.MethodPost, envOr("PAYMENT_URL", "http://payment:8000")+"/pay?order="+orderID); err != nil {
		fail(ctx, w, "payment failed", orderID, err)
		return
	}
	orders.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", "placed"), attribute.String("product", product)))
	log.InfoContext(ctx, "order placed", "order.id", orderID, "product", product)
	_, _ = w.Write([]byte(orderID))
}

func fail(ctx context.Context, w http.ResponseWriter, msg, orderID string, err error) {
	orders.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", "failed")))
	log.ErrorContext(ctx, msg, "order.id", orderID, "error", err)
	http.Error(w, msg, http.StatusBadGateway)
}

// --- inventory --------------------------------------------------------------

var stockLevels [5]atomic.Int64

func registerStockGauge() {
	for i := range stockLevels {
		stockLevels[i].Store(int64(50 + rand.IntN(50)))
	}
	_, _ = meter.Int64ObservableGauge("shop.inventory.stock", metric.WithUnit("{item}"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			for i, p := range products {
				o.Observe(stockLevels[i].Load(), metric.WithAttributes(attribute.String("product", p)))
			}
			return nil
		}))
}

func stock(w http.ResponseWriter, r *http.Request) {
	sleep(r.Context(), "db.query stock", 5, 25)
	_, _ = w.Write([]byte("ok"))
}

func reserve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	product := r.URL.Query().Get("product")
	sleep(ctx, "db.update stock", 10, 40)
	for i, p := range products {
		if p != product {
			continue
		}
		left := stockLevels[i].Add(-1)
		if left < 10 {
			log.WarnContext(ctx, "low stock, restocking", "product", product, "stock", left)
			stockLevels[i].Store(int64(80 + rand.IntN(40)))
		}
	}
	_, _ = w.Write([]byte("ok"))
}

// --- payment ----------------------------------------------------------------

var amounts, _ = meter.Float64Histogram("shop.payment.amount", metric.WithUnit("EUR"),
	metric.WithExplicitBucketBoundaries(1, 2, 5, 10, 20, 50))

// Card issuers. acme-bank is the troublemaker: slow and refusing many
// cards, so that "what do failing payments have in common?" has an answer.
var issuers = []struct {
	name             string
	slowPct, failPct int
}{
	{"acme-bank", 30, 25},
	{"globex", 2, 2},
	{"initech", 2, 2},
}

func pay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.URL.Query().Get("order")
	issuer := issuers[rand.IntN(len(issuers))]
	ctx, span := tracer.Start(ctx, "charge card", trace.WithAttributes(
		attribute.String("order.id", orderID), attribute.String("payment.issuer", issuer.name)))
	defer span.End()
	trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("payment.issuer", issuer.name))

	latency := 20 + rand.IntN(120)
	if rand.IntN(100) < issuer.slowPct {
		latency += 800
		log.WarnContext(ctx, "bank gateway is slow", "order.id", orderID, "payment.issuer", issuer.name, "latency_ms", latency)
	}
	time.Sleep(time.Duration(latency) * time.Millisecond)

	if rand.IntN(100) < issuer.failPct {
		err := errors.New("card declined by issuer")
		if rand.IntN(4) == 0 {
			err = errors.New("card issuer unavailable")
		}
		span.RecordError(err, trace.WithAttributes(attribute.String("exception.type", "CardDeclined")))
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "payment refused", "order.id", orderID, "payment.issuer", issuer.name, "error", err)
		status := http.StatusPaymentRequired
		if err.Error() == "card issuer unavailable" {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}
	amount := 1.5 + rand.Float64()*30
	amounts.Record(ctx, amount)
	log.InfoContext(ctx, "payment accepted", "order.id", orderID, "payment.issuer", issuer.name, "amount_eur", fmt.Sprintf("%.2f", amount))
	_, _ = w.Write([]byte("ok"))
}

// --- helpers ----------------------------------------------------------------

func call(ctx context.Context, method, url string) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s", method, req.URL.Path, resp.Status)
	}
	return nil
}

func sleep(ctx context.Context, name string, minMs, maxMs int) {
	_, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system.name", "postgresql")))
	time.Sleep(time.Duration(minMs+rand.IntN(maxMs-minMs)) * time.Millisecond)
	span.End()
}

// selfURL turns a listen address (":8000" or "127.0.0.1:8001") into a URL.
func selfURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
