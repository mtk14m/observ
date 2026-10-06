package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// generator produces realistic telemetry: a fleet of services running in
// Kubernetes, log lines built from templates with variable parts, traces
// crossing several services, and metric series reported every 10 seconds.
type generator struct {
	rnd      *rand.Rand
	services []service
	start    time.Time
}

type service struct {
	name string
	pods []string
}

var templates = []string{
	"GET %s completed in %dms with status %d",
	"POST %s completed in %dms with status %d",
	"user %d logged in from %s",
	"cache miss for key session:%d",
	"retrying request to %s (attempt %d)",
	"order %d placed for %d items",
	"payment %d refused: card declined by issuer",
	"connection to postgres://db:5432 reset, reconnecting in %dms",
	"processed batch of %d events in %dms",
	"feature flag %s evaluated to %t for user %d",
}

var routes = []string{"/api/cart", "/api/checkout", "/api/products", "/api/users/me", "/api/orders", "/health", "/api/search"}

func newGenerator(seed uint64, nServices int, start time.Time) *generator {
	g := &generator{rnd: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), start: start}
	for i := range nServices {
		s := service{name: fmt.Sprintf("service-%02d", i)}
		for p := range 3 {
			s.pods = append(s.pods, fmt.Sprintf("%s-7f9c%04x-%c%c%c%c%c", s.name, g.rnd.IntN(1<<16), 'a'+p, 'k'+p, 'x', 'q', 'z'))
		}
		g.services = append(g.services, s)
	}
	return g
}

func (g *generator) resource(r pcommon.Resource, s service) {
	a := r.Attributes()
	a.PutStr("service.name", s.name)
	a.PutStr("service.version", "1.4.2")
	a.PutStr("deployment.environment.name", "production")
	a.PutStr("k8s.namespace.name", "shop")
	a.PutStr("k8s.pod.name", s.pods[g.rnd.IntN(len(s.pods))])
	a.PutStr("k8s.node.name", fmt.Sprintf("node-%d", g.rnd.IntN(6)))
	a.PutStr("telemetry.sdk.language", "go")
	a.PutStr("telemetry.sdk.name", "opentelemetry")
	a.PutStr("telemetry.sdk.version", "1.47.0")
}

func (g *generator) at(i, total int, span time.Duration) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(g.start.Add(time.Duration(float64(span) * float64(i) / float64(total))))
}

func (g *generator) traceID() pcommon.TraceID {
	var id pcommon.TraceID
	for i := range id {
		id[i] = byte(g.rnd.IntN(256))
	}
	return id
}

// logs returns a batch of n log records, one resource per service.
func (g *generator) logs(n, offset, total int, span time.Duration) plog.Logs {
	ld := plog.NewLogs()
	per := max(1, n/len(g.services))
	for i, s := range g.services {
		rl := ld.ResourceLogs().AppendEmpty()
		g.resource(rl.Resource(), s)
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("app")
		for j := range per {
			lr := sl.LogRecords().AppendEmpty()
			lr.SetTimestamp(g.at(offset+i*per+j, total, span))
			sev := plog.SeverityNumberInfo
			switch r := g.rnd.IntN(100); {
			case r < 3:
				sev = plog.SeverityNumberError
			case r < 10:
				sev = plog.SeverityNumberWarn
			case r < 30:
				sev = plog.SeverityNumberDebug
			}
			lr.SetSeverityNumber(sev)
			lr.SetSeverityText(sev.String())
			lr.Body().SetStr(g.message())
			if g.rnd.IntN(2) == 0 {
				lr.SetTraceID(g.traceID())
			}
			lr.Attributes().PutStr("http.route", routes[g.rnd.IntN(len(routes))])
			lr.Attributes().PutInt("user.id", int64(g.rnd.IntN(50_000)))
		}
	}
	return ld
}

func (g *generator) message() string {
	switch i := g.rnd.IntN(len(templates)); i {
	case 0, 1:
		return fmt.Sprintf(templates[i], routes[g.rnd.IntN(len(routes))], g.rnd.IntN(900), []int{200, 200, 200, 404, 500}[g.rnd.IntN(5)])
	case 2:
		return fmt.Sprintf(templates[i], g.rnd.IntN(50_000), fmt.Sprintf("10.0.%d.%d", g.rnd.IntN(256), g.rnd.IntN(256)))
	case 3, 6:
		return fmt.Sprintf(templates[i], g.rnd.IntN(1_000_000))
	case 4:
		return fmt.Sprintf(templates[i], routes[g.rnd.IntN(len(routes))], 1+g.rnd.IntN(3))
	case 5:
		return fmt.Sprintf(templates[i], g.rnd.IntN(1_000_000), 1+g.rnd.IntN(9))
	case 7:
		return fmt.Sprintf(templates[i], 100*(1+g.rnd.IntN(10)))
	case 8:
		return fmt.Sprintf(templates[i], g.rnd.IntN(5000), g.rnd.IntN(900))
	default:
		return fmt.Sprintf(templates[i], "new-checkout", g.rnd.IntN(2) == 0, g.rnd.IntN(50_000))
	}
}

// traces returns traces totalling about n spans, each crossing 3-6 services.
func (g *generator) traces(n, offset, total int, span time.Duration) ptrace.Traces {
	td := ptrace.NewTraces()
	for made := 0; made < n; {
		tid := g.traceID()
		start := g.at(offset+made, total, span).AsTime()
		depth := 3 + g.rnd.IntN(4)
		var parent pcommon.SpanID
		failed := g.rnd.IntN(100) < 4
		for d := range depth {
			s := g.services[g.rnd.IntN(len(g.services))]
			rs := td.ResourceSpans().AppendEmpty()
			g.resource(rs.Resource(), s)
			sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			var id pcommon.SpanID
			for i := range id {
				id[i] = byte(g.rnd.IntN(256))
			}
			sp.SetTraceID(tid)
			sp.SetSpanID(id)
			sp.SetParentSpanID(parent)
			route := routes[g.rnd.IntN(len(routes))]
			sp.SetName("GET " + route)
			sp.SetKind(ptrace.SpanKindServer)
			dur := time.Duration(1+g.rnd.IntN(300)) * time.Millisecond
			sp.SetStartTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Duration(d) * time.Millisecond)))
			sp.SetEndTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Duration(d)*time.Millisecond + dur)))
			sp.Attributes().PutStr("http.request.method", "GET")
			sp.Attributes().PutStr("http.route", route)
			sp.Attributes().PutInt("http.response.status_code", 200)
			if failed && d == depth-1 {
				sp.Status().SetCode(ptrace.StatusCodeError)
				sp.Attributes().PutInt("http.response.status_code", 503)
			}
			parent = id
			made++
		}
	}
	return td
}

// metrics returns one report of every series: per service, a gauge, a
// cumulative counter and a latency histogram per route.
func (g *generator) metrics(tick int, at time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ts := pcommon.NewTimestampFromTime(at)
	start := pcommon.NewTimestampFromTime(g.start)
	for si, s := range g.services {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", s.name)
		rm.Resource().Attributes().PutStr("k8s.pod.name", s.pods[0])
		ms := rm.ScopeMetrics().AppendEmpty().Metrics()

		g1 := ms.AppendEmpty()
		g1.SetName("process.memory.usage")
		g1.SetUnit("By")
		dp := g1.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(ts)
		dp.SetIntValue(int64(200_000_000 + g.rnd.IntN(50_000_000)))

		c := ms.AppendEmpty()
		c.SetName("http.server.requests")
		sum := c.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		h := ms.AppendEmpty()
		h.SetName("http.server.request.duration")
		h.SetUnit("s")
		hist := h.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		for ri, route := range routes {
			cp := sum.DataPoints().AppendEmpty()
			cp.SetStartTimestamp(start)
			cp.SetTimestamp(ts)
			cp.SetIntValue(int64((tick + 1) * (10 + si + ri)))
			cp.Attributes().PutStr("http.route", route)

			hp := hist.DataPoints().AppendEmpty()
			hp.SetStartTimestamp(start)
			hp.SetTimestamp(ts)
			hp.Attributes().PutStr("http.route", route)
			hp.ExplicitBounds().FromRaw([]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5})
			count := uint64((tick + 1) * 50)
			hp.SetCount(count)
			hp.SetSum(float64(count) * 0.08)
			hp.BucketCounts().FromRaw([]uint64{count / 10, count / 10, count / 5, count / 5, count / 10, count / 10, count / 10, count / 20, count / 20, 0})
		}
	}
	return md
}
