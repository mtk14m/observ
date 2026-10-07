package notify_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
	"github.com/mtk14n/obsrv/internal/notify"
)

var (
	rule = alert.Rule{ID: "r1", Name: "Checkout errors", Op: ">", Threshold: 5, WindowSeconds: 300}
	ev   = alert.Event{RuleID: "r1", RuleName: "Checkout errors", Status: alert.StatusFiring,
		Labels: map[string]string{"service": "checkout"}, Value: 12, At: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}
)

func capture(t *testing.T, status int) (*httptest.Server, *[]byte) {
	t.Helper()
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request = %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &body
}

func TestWebhookPayload(t *testing.T) {
	srv, body := capture(t, http.StatusOK)
	s := notify.New(notify.Options{BaseURL: "http://obsrv.local:8080"})
	err := s.Send(t.Context(), alert.Channel{Type: alert.ChannelWebhook, URL: srv.URL}, rule, ev)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(*body, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", *body, err)
	}
	want := map[string]any{
		"status": "firing", "rule_id": "r1", "rule_name": "Checkout errors",
		"labels": map[string]any{"service": "checkout"}, "value": 12.0, "op": ">", "threshold": 5.0,
		"window_seconds": 300.0, "at": "2026-10-07T12:00:00Z", "url": "http://obsrv.local:8080/alerts?rule=r1",
	}
	for k, v := range want {
		if gv, _ := json.Marshal(got[k]); string(gv) != mustJSON(v) {
			t.Errorf("%s = %s, want %s", k, gv, mustJSON(v))
		}
	}
}

func TestSlackMessage(t *testing.T) {
	srv, body := capture(t, http.StatusOK)
	s := notify.New(notify.Options{BaseURL: "http://obsrv.local:8080"})
	if err := s.Send(t.Context(), alert.Channel{Type: alert.ChannelSlack, URL: srv.URL}, rule, ev); err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(*body, &msg)
	for _, part := range []string{"FIRING", "Checkout errors", "service=checkout", "12 > 5", "5m", "http://obsrv.local:8080/alerts?rule=r1"} {
		if !strings.Contains(msg.Text, part) {
			t.Errorf("Slack text %q does not contain %q", msg.Text, part)
		}
	}

	resolved := ev
	resolved.Status = alert.StatusResolved
	resolved.Value = 2
	_ = s.Send(t.Context(), alert.Channel{Type: alert.ChannelSlack, URL: srv.URL}, rule, resolved)
	_ = json.Unmarshal(*body, &msg)
	if !strings.Contains(msg.Text, "RESOLVED") {
		t.Errorf("resolved text = %q", msg.Text)
	}
}

func TestErrorsAreReported(t *testing.T) {
	srv, _ := capture(t, http.StatusInternalServerError)
	s := notify.New(notify.Options{})
	if err := s.Send(t.Context(), alert.Channel{Type: alert.ChannelWebhook, URL: srv.URL}, rule, ev); err == nil {
		t.Error("Send to a failing endpoint returned nil error")
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
