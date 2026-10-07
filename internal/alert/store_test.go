package alert_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
	"github.com/mtk14n/obsrv/internal/metadata"
)

func newStore(t *testing.T) *alert.Store {
	t.Helper()
	db, err := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"), alert.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return alert.NewStore(db)
}

func validRule() alert.Rule {
	return alert.Rule{
		Name: "Checkout errors", Kind: alert.KindLogs, Query: "service:checkout level:error",
		Op: ">", Threshold: 5, WindowSeconds: 300, Enabled: true,
	}
}

func TestRuleCRUD(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	ch, err := s.CreateChannel(ctx, alert.Channel{Name: "ops", Type: alert.ChannelWebhook, URL: "https://example.com/hook"})
	if err != nil {
		t.Fatal(err)
	}

	r := validRule()
	r.Channels = []string{ch.ID}
	created, err := s.CreateRule(ctx, r)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Errorf("created = %+v, want an ID and timestamps", created)
	}

	got, err := s.GetRule(ctx, created.ID)
	if err != nil || got.Name != "Checkout errors" || got.Channels[0] != ch.ID {
		t.Errorf("GetRule = %+v, %v", got, err)
	}

	got.Threshold = 10
	updated, err := s.UpdateRule(ctx, created.ID, got)
	if err != nil || updated.Threshold != 10 || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("UpdateRule = %+v, %v", updated, err)
	}

	list, err := s.ListRules(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("ListRules = %v, %v", list, err)
	}

	if err := s.DeleteRule(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRule(ctx, created.ID); !errors.Is(err, alert.ErrNotFound) {
		t.Errorf("GetRule after delete = %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateRule(ctx, "missing", validRule()); !errors.Is(err, alert.ErrNotFound) {
		t.Errorf("UpdateRule(missing) = %v, want ErrNotFound", err)
	}
}

func TestRuleValidation(t *testing.T) {
	cases := map[string]func(*alert.Rule){
		"no name":            func(r *alert.Rule) { r.Name = " " },
		"unknown kind":       func(r *alert.Rule) { r.Kind = "magic" },
		"metric without one": func(r *alert.Rule) { r.Kind = alert.KindMetric; r.Metric = "" },
		"unknown operator":   func(r *alert.Rule) { r.Op = "≈" },
		"window too short":   func(r *alert.Rule) { r.WindowSeconds = 5 },
		"negative for":       func(r *alert.Rule) { r.ForSeconds = -1 },
		"unknown channel":    func(r *alert.Rule) { r.Channels = []string{"nope"} },
		"bad log query":      func(r *alert.Rule) { r.Query = `"unterminated` },
	}
	s := newStore(t)
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRule()
			mutate(&r)
			if _, err := s.CreateRule(t.Context(), r); !errors.Is(err, alert.ErrInvalid) {
				t.Errorf("CreateRule = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestChannels(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	if _, err := s.CreateChannel(ctx, alert.Channel{Name: "x", Type: "pigeon", URL: "https://x"}); !errors.Is(err, alert.ErrInvalid) {
		t.Errorf("unknown type: %v, want ErrInvalid", err)
	}
	if _, err := s.CreateChannel(ctx, alert.Channel{Name: "x", Type: alert.ChannelSlack, URL: "not a url"}); !errors.Is(err, alert.ErrInvalid) {
		t.Errorf("bad URL: %v, want ErrInvalid", err)
	}
	ch, err := s.CreateChannel(ctx, alert.Channel{Name: "slack", Type: alert.ChannelSlack, URL: "https://hooks.slack.com/services/x"})
	if err != nil {
		t.Fatal(err)
	}
	r := validRule()
	r.Channels = []string{ch.ID}
	rule, _ := s.CreateRule(ctx, r)

	if err := s.DeleteChannel(ctx, ch.ID); !errors.Is(err, alert.ErrInUse) {
		t.Errorf("deleting a channel used by a rule = %v, want ErrInUse", err)
	}
	_ = s.DeleteRule(ctx, rule.ID)
	if err := s.DeleteChannel(ctx, ch.ID); err != nil {
		t.Errorf("DeleteChannel: %v", err)
	}
}

func TestStatesFollowTheirRule(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	rule, _ := s.CreateRule(ctx, validRule())
	states := []alert.State{{RuleID: rule.ID, Group: "service=checkout", Labels: map[string]string{"service": "checkout"},
		Status: alert.StatusFiring, Since: time.Unix(100, 0).UTC(), Value: 12}}
	if err := s.ReplaceStates(ctx, rule.ID, states); err != nil {
		t.Fatal(err)
	}
	got, err := s.States(ctx)
	if err != nil || len(got) != 1 || got[0].Status != alert.StatusFiring || got[0].Value != 12 {
		t.Errorf("States = %+v, %v", got, err)
	}
	_ = s.DeleteRule(ctx, rule.ID)
	if got, _ := s.States(ctx); len(got) != 0 {
		t.Errorf("states of a deleted rule must go: %+v", got)
	}
}

func TestEventsNewestFirst(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	for i := range 3 {
		if err := s.AddEvents(ctx, []alert.Event{{RuleID: "r", RuleName: "R", Status: alert.StatusFiring,
			Value: float64(i), At: time.Unix(int64(i), 0)}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Events(ctx, 2)
	if err != nil || len(got) != 2 || got[0].Value != 2 || got[1].Value != 1 {
		t.Errorf("Events = %+v, %v", got, err)
	}
}
