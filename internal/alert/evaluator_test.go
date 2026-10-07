package alert_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
	"github.com/mtk14n/obsrv/internal/query"
)

type fakeSource struct {
	counts []query.GroupCount
	series []query.Series
	err    error
	last   any
}

func (f *fakeSource) CountLogs(_ context.Context, q query.LogQuery, groupBy []string) ([]query.GroupCount, error) {
	f.last = []any{q, groupBy}
	return f.counts, f.err
}

func (f *fakeSource) QueryMetric(_ context.Context, q query.MetricQuery) ([]query.Series, error) {
	f.last = q
	return f.series, f.err
}

type fakeSender struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeSender) Send(_ context.Context, ch alert.Channel, _ alert.Rule, ev alert.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, ch.Name+":"+ev.Status)
	return nil
}

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func evaluator(t *testing.T, src *fakeSource) (*alert.Evaluator, *alert.Store, *fakeSender) {
	t.Helper()
	store := newStore(t)
	sender := &fakeSender{}
	e := alert.NewEvaluator(alert.EvaluatorOptions{Store: store, Source: src, Sender: sender, Now: func() time.Time { return now }})
	return e, store, sender
}

func TestLogRuleFiresAndNotifiesOnce(t *testing.T) {
	src := &fakeSource{counts: []query.GroupCount{{Labels: map[string]string{}, Count: 12}}}
	e, store, sender := evaluator(t, src)
	ctx := t.Context()
	ch, _ := store.CreateChannel(ctx, alert.Channel{Name: "ops", Type: alert.ChannelWebhook, URL: "https://example.com"})
	r := validRule()
	r.Channels = []string{ch.ID}
	rule, _ := store.CreateRule(ctx, r)

	if err := e.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	q := src.last.([]any)[0].(query.LogQuery)
	if q.Query != "service:checkout level:error" || !q.To.Equal(now) || !q.From.Equal(now.Add(-5*time.Minute)) {
		t.Errorf("log query = %+v", q)
	}
	states, _ := store.States(ctx)
	if len(states) != 1 || states[0].RuleID != rule.ID || states[0].Status != alert.StatusFiring {
		t.Errorf("states = %+v", states)
	}
	events, _ := store.Events(ctx, 10)
	if len(events) != 1 || events[0].Status != alert.StatusFiring {
		t.Errorf("events = %+v", events)
	}

	_ = e.RunOnce(ctx)
	if len(sender.sent) != 1 || sender.sent[0] != "ops:firing" {
		t.Errorf("sent = %v, want a single firing notification", sender.sent)
	}

	src.counts[0].Count = 0
	_ = e.RunOnce(ctx)
	if len(sender.sent) != 2 || sender.sent[1] != "ops:resolved" {
		t.Errorf("sent = %v, want firing then resolved", sender.sent)
	}
}

func TestMetricRuleEvaluatesTheWindowPerGroup(t *testing.T) {
	src := &fakeSource{series: []query.Series{
		{Labels: map[string]string{"service.name": "checkout"}, Points: []query.Point{{T: 1, V: 0.9}}},
		{Labels: map[string]string{"service.name": "frontend"}, Points: []query.Point{{T: 1, V: 0.1}}},
	}}
	e, store, _ := evaluator(t, src)
	ctx := t.Context()
	_, err := store.CreateRule(ctx, alert.Rule{
		Name: "Slow", Kind: alert.KindMetric, Metric: "http.server.request.duration", Agg: "p95",
		GroupBy: []string{"service.name"}, Op: ">", Threshold: 0.5, WindowSeconds: 300, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	q := src.last.(query.MetricQuery)
	if q.Metric != "http.server.request.duration" || q.Agg != "p95" || q.Step != 5*time.Minute || !q.From.Equal(now.Add(-5*time.Minute)) {
		t.Errorf("metric query = %+v", q)
	}
	firing := 0
	states, _ := store.States(ctx)
	for _, s := range states {
		if s.Status == alert.StatusFiring {
			firing++
			if s.Labels["service.name"] != "checkout" {
				t.Errorf("firing group = %v", s.Labels)
			}
		}
	}
	if firing != 1 || len(states) != 2 {
		t.Errorf("states = %+v", states)
	}
}

func TestDisabledRulesAreNotEvaluated(t *testing.T) {
	src := &fakeSource{counts: []query.GroupCount{{Labels: map[string]string{}, Count: 99}}}
	e, store, _ := evaluator(t, src)
	r := validRule()
	r.Enabled = false
	_, _ = store.CreateRule(t.Context(), r)
	_ = e.RunOnce(t.Context())
	if src.last != nil {
		t.Error("a disabled rule was evaluated")
	}
}

func TestOneFailingRuleDoesNotStopTheOthers(t *testing.T) {
	src := &fakeSource{err: errors.New("disk on fire")}
	e, store, _ := evaluator(t, src)
	_, _ = store.CreateRule(t.Context(), validRule())
	if err := e.RunOnce(t.Context()); err == nil {
		t.Error("RunOnce should report the failure")
	}
}

func TestPreview(t *testing.T) {
	src := &fakeSource{counts: []query.GroupCount{{Labels: map[string]string{"service.name": "api"}, Count: 7}}}
	e, _, _ := evaluator(t, src)
	values, err := e.Values(t.Context(), validRule())
	if err != nil || len(values) != 1 || values[0].Value != 7 || values[0].Labels["service.name"] != "api" {
		t.Errorf("Values = %+v, %v", values, err)
	}
}
