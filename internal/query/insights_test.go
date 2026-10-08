package query_test

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mtk14n/obsrv/internal/ingest"
	"github.com/mtk14n/obsrv/internal/objstore/fs"
	"github.com/mtk14n/obsrv/internal/query"
)

func TestCompareErrorsWithTheRest(t *testing.T) {
	got, err := fixture(t).Compare(t.Context(), query.CompareQuery{TimeRange: window, Signal: "spans", Errors: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectionTotal != 1 || got.BaselineTotal != 2 {
		t.Fatalf("totals = %d/%d, want 1/2", got.SelectionTotal, got.BaselineTotal)
	}
	top := got.Items[0]
	if top.Key != "service.name" || top.Value != "payment" || top.Selection != 1 || top.Baseline != 0 {
		t.Errorf("top difference = %+v, want service.name=payment 100%% vs 0%%", top)
	}
	for _, it := range got.Items {
		if it.Selection <= it.Baseline {
			t.Errorf("item %+v is not over-represented in the selection", it)
		}
	}
}

func TestCompareATimeWindowWithTheRest(t *testing.T) {
	sel := query.TimeRange{From: base.Add(19 * time.Minute), To: base.Add(21 * time.Minute)}
	got, err := fixture(t).Compare(t.Context(), query.CompareQuery{TimeRange: window, Signal: "spans", Window: sel})
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectionTotal != 1 || got.Items[0].Key != "http.route" || got.Items[0].Value != "GET /home" {
		t.Errorf("window compare = %+v", got)
	}
}

func TestCompareLogs(t *testing.T) {
	got, err := fixture(t).Compare(t.Context(), query.CompareQuery{TimeRange: window, Signal: "logs", Errors: true})
	if err != nil || got.SelectionTotal != 1 || got.Items[0].Value != "payment" {
		t.Errorf("logs compare = %+v, %v", got, err)
	}
	if _, err := fixture(t).Compare(t.Context(), query.CompareQuery{TimeRange: window, Signal: "nope", Errors: true}); err == nil {
		t.Error("unknown signal returned nil error")
	}
}

func TestIssues(t *testing.T) {
	issues, err := fixture(t).Issues(t.Context(), window, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %+v, want one log issue and one span issue", issues)
	}
	byKind := map[string]query.Issue{}
	for _, is := range issues {
		byKind[is.Kind] = is
		// The fixture has no data before the window: nothing is called new.
		if is.Status != "ongoing" || is.Count != 1 || is.PreviousCount != 0 || len(is.Buckets) != 6 || is.ID == "" {
			t.Errorf("issue = %+v", is)
		}
	}
	if l := byKind["log"]; l.Service != "payment" || l.Title != "card declined" || l.ExampleTraceID != trace1H {
		t.Errorf("log issue = %+v", l)
	}
	if s := byKind["span"]; s.Service != "payment" || s.Title != "POST /pay" {
		t.Errorf("span issue = %+v", s)
	}
}

func TestIssueTemplatesHideVariableParts(t *testing.T) {
	got := query.Template("order 123456 for user 9f86d081884c7d65 failed after 2.5s")
	if got != "order <*> for user <*> failed after <*>s" {
		t.Errorf("Template = %q", got)
	}
}

func TestDeployments(t *testing.T) {
	store, _ := fs.New(t.TempDir())
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	td := ptrace.NewTraces()
	add := func(service, version string, at time.Duration) {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", service)
		rs.Resource().Attributes().PutStr("service.version", version)
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(base.Add(at)))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(at + time.Millisecond)))
	}
	add("checkout", "1.4.1", -2*time.Hour) // running before the window
	add("checkout", "1.4.2", 10*time.Minute)
	add("brand-new", "0.1.0", 15*time.Minute) // no history: not a deployment
	if err := p.ConsumeTraces(t.Context(), td); err != nil {
		t.Fatal(err)
	}
	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	e, _ := query.New(store)
	defer func() { _ = e.Close() }()

	got, err := e.Deployments(t.Context(), window)
	if err != nil {
		t.Fatal(err)
	}
	want := query.Deployment{Service: "checkout", Version: "1.4.2", Previous: "1.4.1", At: base.Add(10 * time.Minute).UnixNano()}
	if len(got) != 1 || got[0] != want {
		t.Errorf("deployments = %+v, want %+v", got, want)
	}
}

// errorChain writes, at +offset, a trace where an error propagates from
// payment (origin) to checkout and frontend.
func errorChain(t *testing.T, p *ingest.Pipeline, tid byte, offset time.Duration) {
	t.Helper()
	writeChain(t, p, tid, offset, []string{"frontend", "checkout", "payment"}, nil)
}

// writeChain writes a trace whose spans form a chain, all in error except
// the indexes listed in ok.
func writeChain(t *testing.T, p *ingest.Pipeline, tid byte, offset time.Duration, chain []string, ok map[int]bool) {
	t.Helper()
	td := ptrace.NewTraces()
	for i, svc := range chain {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", svc)
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetTraceID(pcommon.TraceID{tid})
		s.SetSpanID(pcommon.SpanID{byte(i + 1)})
		if i > 0 {
			s.SetParentSpanID(pcommon.SpanID{byte(i)})
		}
		s.SetName("call " + svc)
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(base.Add(offset)))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(offset + time.Millisecond)))
		if !ok[i] {
			s.Status().SetCode(ptrace.StatusCodeError)
		}
	}
	if err := p.ConsumeTraces(t.Context(), td); err != nil {
		t.Fatal(err)
	}
}

func insightsEngine(t *testing.T, write func(p *ingest.Pipeline)) *query.Engine {
	t.Helper()
	store, _ := fs.New(t.TempDir())
	p, err := ingest.New(ingest.Options{WALDir: t.TempDir(), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	write(p)
	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	e, _ := query.New(store)
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestIssuesKeepOnlyTheOriginOfPropagatedErrors(t *testing.T) {
	e := insightsEngine(t, func(p *ingest.Pipeline) { errorChain(t, p, 1, 10*time.Minute) })
	issues, err := e.Issues(t.Context(), window, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Service != "payment" || issues[0].Title != "call payment" {
		t.Errorf("issues = %+v, want only the payment span where the error started", issues)
	}
}

func TestIssuesDoNotClaimNewWithoutHistory(t *testing.T) {
	// No data at all in the previous period: nothing can be called new.
	e := insightsEngine(t, func(p *ingest.Pipeline) { errorChain(t, p, 1, 10*time.Minute) })
	issues, _ := e.Issues(t.Context(), window, 6)
	if issues[0].Status != "ongoing" {
		t.Errorf("status = %q, want ongoing when there is no history", issues[0].Status)
	}

	// With history, an error absent from the previous period is new.
	e = insightsEngine(t, func(p *ingest.Pipeline) {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "frontend")
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(base.Add(-30 * time.Minute)))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(base.Add(-30 * time.Minute)))
		_ = p.ConsumeTraces(t.Context(), td)
		errorChain(t, p, 1, 10*time.Minute)
	})
	issues, _ = e.Issues(t.Context(), window, 6)
	if issues[0].Status != "new" {
		t.Errorf("status = %q, want new", issues[0].Status)
	}
}

func TestIssuesFindTheOriginThroughSpansThatAreNotInError(t *testing.T) {
	// An HTTP 4xx is not an error on the server span, so the chain of errors
	// is broken: frontend ✗ → checkout client ✗ → payment server ✓ → charge card ✗.
	e := insightsEngine(t, func(p *ingest.Pipeline) {
		writeChain(t, p, 1, 10*time.Minute, []string{"frontend", "checkout", "payment", "payment"}, map[int]bool{2: true})
	})
	issues, err := e.Issues(t.Context(), window, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Title != "call payment" {
		t.Errorf("issues = %+v, want only the deepest span in error", issues)
	}
}
