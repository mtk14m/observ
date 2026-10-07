package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
)

// Alerts enables the alerting endpoints under /api/v1/alerts.
type Alerts struct {
	Store *alert.Store
	// Previewer computes a rule's current values without saving it.
	Previewer interface {
		Values(ctx context.Context, r alert.Rule) ([]alert.Value, error)
	}
	// Sender delivers test notifications.
	Sender alert.Sender
}

// RuleStatus is a rule with the state of its groups.
type RuleStatus struct {
	alert.Rule
	// Status is the worst status of the rule's groups: firing, pending or ok.
	Status string        `json:"status"`
	Firing int           `json:"firing"`
	States []alert.State `json:"states"`
}

// PreviewValue is a group's current value and whether it breaches the rule.
type PreviewValue struct {
	alert.Value
	Breached bool `json:"breached"`
}

func (a *Alerts) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/alerts/rules", a.listRules)
	mux.HandleFunc("POST /api/v1/alerts/rules", a.createRule)
	mux.HandleFunc("GET /api/v1/alerts/rules/{id}", a.getRule)
	mux.HandleFunc("PUT /api/v1/alerts/rules/{id}", a.updateRule)
	mux.HandleFunc("DELETE /api/v1/alerts/rules/{id}", a.deleteRule)
	mux.HandleFunc("POST /api/v1/alerts/preview", a.preview)
	mux.HandleFunc("GET /api/v1/alerts/channels", a.listChannels)
	mux.HandleFunc("POST /api/v1/alerts/channels", a.createChannel)
	mux.HandleFunc("PUT /api/v1/alerts/channels/{id}", a.updateChannel)
	mux.HandleFunc("DELETE /api/v1/alerts/channels/{id}", a.deleteChannel)
	mux.HandleFunc("POST /api/v1/alerts/channels/{id}/test", a.testChannel)
	mux.HandleFunc("GET /api/v1/alerts/events", a.events)
}

func (a *Alerts) withStates(ctx context.Context, rules []alert.Rule) ([]RuleStatus, error) {
	states, err := a.Store.States(ctx)
	if err != nil {
		return nil, err
	}
	byRule := map[string][]alert.State{}
	for _, s := range states {
		byRule[s.RuleID] = append(byRule[s.RuleID], s)
	}
	out := make([]RuleStatus, len(rules))
	for i, r := range rules {
		rs := RuleStatus{Rule: r, Status: alert.StatusOK, States: byRule[r.ID]}
		if rs.States == nil {
			rs.States = []alert.State{}
		}
		for _, s := range rs.States {
			switch s.Status {
			case alert.StatusFiring:
				rs.Firing++
				rs.Status = alert.StatusFiring
			case alert.StatusPending:
				if rs.Status == alert.StatusOK {
					rs.Status = alert.StatusPending
				}
			}
		}
		out[i] = rs
	}
	return out, nil
}

func (a *Alerts) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := a.Store.ListRules(r.Context())
	if err != nil {
		writeAlertError(w, err)
		return
	}
	out, err := a.withStates(r.Context(), rules)
	write(w, out, err)
}

func (a *Alerts) getRule(w http.ResponseWriter, r *http.Request) {
	rule, err := a.Store.GetRule(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAlertError(w, err)
		return
	}
	out, err := a.withStates(r.Context(), []alert.Rule{rule})
	if err != nil {
		writeAlertError(w, err)
		return
	}
	write(w, out[0], nil)
}

func (a *Alerts) createRule(w http.ResponseWriter, r *http.Request) {
	var rule alert.Rule
	if !decode(w, r, &rule) {
		return
	}
	created, err := a.Store.CreateRule(r.Context(), rule)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (a *Alerts) updateRule(w http.ResponseWriter, r *http.Request) {
	var rule alert.Rule
	if !decode(w, r, &rule) {
		return
	}
	updated, err := a.Store.UpdateRule(r.Context(), r.PathValue("id"), rule)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	write(w, updated, nil)
}

func (a *Alerts) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteRule(r.Context(), r.PathValue("id")); err != nil {
		writeAlertError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Alerts) preview(w http.ResponseWriter, r *http.Request) {
	var rule alert.Rule
	if !decode(w, r, &rule) {
		return
	}
	rule.Name = "preview"
	if err := rule.Validate(); err != nil {
		writeAlertError(w, err)
		return
	}
	values, err := a.Previewer.Values(r.Context(), rule)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	out := make([]PreviewValue, len(values))
	for i, v := range values {
		out[i] = PreviewValue{Value: v, Breached: rule.Breached(v.Value)}
	}
	write(w, out, nil)
}

func (a *Alerts) listChannels(w http.ResponseWriter, r *http.Request) {
	chans, err := a.Store.ListChannels(r.Context())
	write(w, chans, err)
}

func (a *Alerts) createChannel(w http.ResponseWriter, r *http.Request) {
	var ch alert.Channel
	if !decode(w, r, &ch) {
		return
	}
	created, err := a.Store.CreateChannel(r.Context(), ch)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (a *Alerts) updateChannel(w http.ResponseWriter, r *http.Request) {
	var ch alert.Channel
	if !decode(w, r, &ch) {
		return
	}
	updated, err := a.Store.UpdateChannel(r.Context(), r.PathValue("id"), ch)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	write(w, updated, nil)
}

func (a *Alerts) deleteChannel(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteChannel(r.Context(), r.PathValue("id")); err != nil {
		writeAlertError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testChannel sends a sample firing notification to check a channel works.
func (a *Alerts) testChannel(w http.ResponseWriter, r *http.Request) {
	ch, err := a.Store.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAlertError(w, err)
		return
	}
	rule := alert.Rule{ID: "test", Name: "Test notification from obsrv", Op: ">", Threshold: 0, WindowSeconds: 300}
	ev := alert.Event{RuleID: "test", RuleName: rule.Name, Status: alert.StatusFiring,
		Labels: map[string]string{"channel": ch.Name}, Value: 1, At: time.Now()}
	if err := a.Sender.Send(r.Context(), ch, rule, ev); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	write(w, map[string]string{"result": "sent"}, nil)
}

func (a *Alerts) events(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 || n > 1000 {
			writeError(w, fmt.Errorf("%w: limit must be between 1 and 1000", errBadRequest))
			return
		}
		limit = n
	}
	events, err := a.Store.Events(r.Context(), limit)
	write(w, events, err)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err == nil {
		err = json.Unmarshal(body, v)
	}
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid JSON body: %w", errBadRequest, err))
		return false
	}
	return true
}

func writeAlertError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, alert.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, alert.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, alert.ErrInUse):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		writeError(w, err)
	}
}
