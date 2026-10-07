package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
	"github.com/mtk14n/obsrv/internal/api"
	"github.com/mtk14n/obsrv/internal/metadata"
)

type fakePreviewer struct{}

func (fakePreviewer) Values(context.Context, alert.Rule) ([]alert.Value, error) {
	return []alert.Value{{Labels: map[string]string{"service.name": "api"}, Value: 9}}, nil
}

type fakeTester struct{ sent []string }

func (f *fakeTester) Send(_ context.Context, ch alert.Channel, _ alert.Rule, ev alert.Event) error {
	f.sent = append(f.sent, ch.Name+":"+ev.Status)
	return nil
}

type alertsEnv struct {
	h      http.Handler
	store  *alert.Store
	tester *fakeTester
}

func newAlertsEnv(t *testing.T) alertsEnv {
	t.Helper()
	db, err := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"))
	if err == nil {
		err = metadata.Migrate(t.Context(), db, "alert", alert.Migrations)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := alert.NewStore(db)
	tester := &fakeTester{}
	h := api.NewHandler(&fakeQuerier{}, api.Options{
		Now:    func() time.Time { return now },
		Alerts: &api.Alerts{Store: store, Previewer: fakePreviewer{}, Sender: tester},
	})
	return alertsEnv{h: h, store: store, tester: tester}
}

func (e alertsEnv) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(b)))
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: invalid JSON %q", method, path, rec.Body.String())
		}
	}
	return rec.Code, out
}

var ruleBody = map[string]any{
	"name": "Checkout errors", "kind": "logs", "query": "service:checkout level:error",
	"op": ">", "threshold": 5, "window_seconds": 300, "enabled": true,
}

func TestAlertRulesLifecycle(t *testing.T) {
	e := newAlertsEnv(t)
	code, body := e.do(t, http.MethodPost, "/api/v1/alerts/rules", ruleBody)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, body)
	}
	id := body["data"].(map[string]any)["id"].(string)

	code, body = e.do(t, http.MethodGet, "/api/v1/alerts/rules", nil)
	list := body["data"].([]any)
	if code != http.StatusOK || len(list) != 1 || list[0].(map[string]any)["status"] != "ok" {
		t.Errorf("list: %d %v", code, body)
	}

	updated := map[string]any{}
	for k, v := range ruleBody {
		updated[k] = v
	}
	updated["threshold"] = 10
	if code, body := e.do(t, http.MethodPut, "/api/v1/alerts/rules/"+id, updated); code != http.StatusOK ||
		body["data"].(map[string]any)["threshold"] != 10.0 {
		t.Errorf("update: %d %v", code, body)
	}
	if code, _ := e.do(t, http.MethodDelete, "/api/v1/alerts/rules/"+id, nil); code != http.StatusNoContent {
		t.Errorf("delete: %d", code)
	}
	if code, _ := e.do(t, http.MethodGet, "/api/v1/alerts/rules/"+id, nil); code != http.StatusNotFound {
		t.Errorf("get deleted: %d", code)
	}
}

func TestInvalidRuleIsExplained(t *testing.T) {
	e := newAlertsEnv(t)
	bad := map[string]any{"name": "", "kind": "logs", "op": ">", "window_seconds": 5}
	code, body := e.do(t, http.MethodPost, "/api/v1/alerts/rules", bad)
	if code != http.StatusBadRequest || body["error"] == nil {
		t.Errorf("invalid rule: %d %v", code, body)
	}
}

func TestRuleStatusReflectsFiringGroups(t *testing.T) {
	e := newAlertsEnv(t)
	_, body := e.do(t, http.MethodPost, "/api/v1/alerts/rules", ruleBody)
	id := body["data"].(map[string]any)["id"].(string)
	_ = e.store.ReplaceStates(t.Context(), id, []alert.State{
		{RuleID: id, Group: "a", Status: alert.StatusFiring, Value: 9},
		{RuleID: id, Group: "b", Status: alert.StatusOK},
	})
	_, body = e.do(t, http.MethodGet, "/api/v1/alerts/rules", nil)
	r := body["data"].([]any)[0].(map[string]any)
	if r["status"] != "firing" || r["firing"] != 1.0 || len(r["states"].([]any)) != 2 {
		t.Errorf("rule status = %v", r)
	}
}

func TestPreviewEvaluatesWithoutSaving(t *testing.T) {
	e := newAlertsEnv(t)
	code, body := e.do(t, http.MethodPost, "/api/v1/alerts/preview", ruleBody)
	if code != http.StatusOK {
		t.Fatalf("preview: %d %v", code, body)
	}
	v := body["data"].([]any)[0].(map[string]any)
	if v["value"] != 9.0 || v["breached"] != true {
		t.Errorf("preview = %v", v)
	}
	if rules, _ := e.store.ListRules(t.Context()); len(rules) != 0 {
		t.Error("preview must not save the rule")
	}
}

func TestChannelsAndTestNotification(t *testing.T) {
	e := newAlertsEnv(t)
	code, body := e.do(t, http.MethodPost, "/api/v1/alerts/channels",
		map[string]any{"name": "ops", "type": "slack", "url": "https://hooks.slack.com/services/x"})
	if code != http.StatusCreated {
		t.Fatalf("create channel: %d %v", code, body)
	}
	id := body["data"].(map[string]any)["id"].(string)

	if code, _ := e.do(t, http.MethodPost, "/api/v1/alerts/channels/"+id+"/test", nil); code != http.StatusOK ||
		len(e.tester.sent) != 1 || e.tester.sent[0] != "ops:firing" {
		t.Errorf("test notification: %d %v", code, e.tester.sent)
	}

	withChannel := map[string]any{}
	for k, v := range ruleBody {
		withChannel[k] = v
	}
	withChannel["channels"] = []string{id}
	e.do(t, http.MethodPost, "/api/v1/alerts/rules", withChannel)
	if code, _ := e.do(t, http.MethodDelete, "/api/v1/alerts/channels/"+id, nil); code != http.StatusConflict {
		t.Errorf("delete used channel: %d, want 409", code)
	}
}

func TestEvents(t *testing.T) {
	e := newAlertsEnv(t)
	_ = e.store.AddEvents(t.Context(), []alert.Event{{RuleID: "r", RuleName: "R", Status: alert.StatusFiring, At: now}})
	code, body := e.do(t, http.MethodGet, "/api/v1/alerts/events", nil)
	if code != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Errorf("events: %d %v", code, body)
	}
}

func TestAlertRoutesAbsentWithoutAlerting(t *testing.T) {
	h := api.NewHandler(&fakeQuerier{}, api.Options{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/alerts/rules", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
