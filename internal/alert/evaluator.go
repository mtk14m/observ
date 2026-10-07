package alert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mtk14n/obsrv/internal/query"
)

// Source computes the values rules are evaluated on. *query.Engine
// implements it.
type Source interface {
	QueryMetric(ctx context.Context, q query.MetricQuery) ([]query.Series, error)
	CountLogs(ctx context.Context, q query.LogQuery, groupBy []string) ([]query.GroupCount, error)
}

// Sender delivers notifications. *notify.Sender implements it.
type Sender interface {
	Send(ctx context.Context, ch Channel, rule Rule, ev Event) error
}

// EvaluatorOptions configures an Evaluator.
type EvaluatorOptions struct {
	Store  *Store
	Source Source
	Sender Sender
	Now    func() time.Time
	Logger *slog.Logger
}

// Evaluator periodically evaluates every enabled rule.
type Evaluator struct {
	opts EvaluatorOptions
}

// NewEvaluator returns an Evaluator.
func NewEvaluator(opts EvaluatorOptions) *Evaluator {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return &Evaluator{opts: opts}
}

// Run evaluates rules every interval until ctx is done.
func (e *Evaluator) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := e.RunOnce(ctx); err != nil && ctx.Err() == nil {
			e.opts.Logger.Error("alert evaluation failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunOnce evaluates every enabled rule once. A failing rule does not stop
// the others; all failures are returned together.
func (e *Evaluator) RunOnce(ctx context.Context) error {
	rules, err := e.opts.Store.ListRules(ctx)
	if err != nil {
		return err
	}
	states, err := e.opts.Store.States(ctx)
	if err != nil {
		return err
	}
	prev := map[string]map[string]State{}
	for _, s := range states {
		if prev[s.RuleID] == nil {
			prev[s.RuleID] = map[string]State{}
		}
		prev[s.RuleID][s.Group] = s
	}

	var errs []error
	for _, r := range rules {
		if err := e.evaluate(ctx, r, prev[r.ID]); err != nil {
			errs = append(errs, fmt.Errorf("rule %q: %w", r.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (e *Evaluator) evaluate(ctx context.Context, r Rule, prev map[string]State) error {
	if !r.Enabled {
		return e.opts.Store.ReplaceStates(ctx, r.ID, nil)
	}
	values, err := e.Values(ctx, r)
	if err != nil {
		return err
	}
	states, events := transition(r, prev, values, e.opts.Now())
	if err := e.opts.Store.ReplaceStates(ctx, r.ID, states); err != nil {
		return err
	}
	if err := e.opts.Store.AddEvents(ctx, events); err != nil {
		return err
	}
	for _, ev := range events {
		e.opts.Logger.Info("alert", "rule", r.Name, "status", ev.Status, "labels", ev.Labels, "value", ev.Value)
		for _, id := range r.Channels {
			ch, err := e.opts.Store.GetChannel(ctx, id)
			if err == nil {
				err = e.opts.Sender.Send(ctx, ch, r, ev)
			}
			if err != nil {
				// A broken channel must not block alerting: log and move on.
				e.opts.Logger.Error("notification failed", "rule", r.Name, "channel", id, "err", err)
			}
		}
	}
	return nil
}

// Values computes the current value of every group of a rule, over the
// window ending now. It also serves rule previews.
func (e *Evaluator) Values(ctx context.Context, r Rule) ([]Value, error) {
	now := e.opts.Now()
	tr := query.TimeRange{From: now.Add(-r.Window()), To: now}
	switch r.Kind {
	case KindLogs:
		counts, err := e.opts.Source.CountLogs(ctx, query.LogQuery{TimeRange: tr, Query: r.Query}, r.GroupBy)
		if err != nil {
			return nil, err
		}
		values := make([]Value, len(counts))
		for i, c := range counts {
			values[i] = Value{Labels: c.Labels, Value: float64(c.Count)}
		}
		return values, nil
	case KindMetric:
		series, err := e.opts.Source.QueryMetric(ctx, query.MetricQuery{
			TimeRange: tr, Metric: r.Metric, Agg: r.Agg, GroupBy: r.GroupBy, Filters: r.Filters, Step: r.Window(),
		})
		if err != nil {
			return nil, err
		}
		values := make([]Value, 0, len(series))
		for _, s := range series {
			if len(s.Points) == 0 {
				continue
			}
			labels := s.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			values = append(values, Value{Labels: labels, Value: s.Points[len(s.Points)-1].V})
		}
		return values, nil
	default:
		return nil, fmt.Errorf("%w: unknown kind %q", ErrInvalid, r.Kind)
	}
}
