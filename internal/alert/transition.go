package alert

import (
	"slices"
	"strings"
	"time"
)

// Value is the current value of one group of a rule.
type Value struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// groupKey identifies a group by its sorted labels, e.g. "service=api".
func groupKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + labels[k]
	}
	return strings.Join(parts, ",")
}

// transition computes the new states of a rule's groups from their previous
// states and current values, and the events to notify:
//
//	ok      --breach-->               firing (For = 0) or pending
//	pending --breach for For-->       firing   (event "firing")
//	pending --no breach-->            ok
//	firing  --no breach or no data--> ok       (event "resolved")
func transition(r Rule, prev map[string]State, values []Value, now time.Time) ([]State, []Event) {
	var states []State
	var events []Event
	seen := map[string]bool{}
	event := func(status string, labels map[string]string, v float64) {
		events = append(events, Event{RuleID: r.ID, RuleName: r.Name, Labels: labels, Status: status, Value: v, At: now})
	}

	for _, v := range values {
		g := groupKey(v.Labels)
		seen[g] = true
		p, known := prev[g]
		s := State{RuleID: r.ID, Group: g, Labels: v.Labels, Value: v.Value, Status: StatusOK, Since: now}
		if known {
			s.Status, s.Since = p.Status, p.Since
		}

		switch breached := r.Breached(v.Value); {
		case breached && s.Status == StatusFiring:
			// Still firing: already notified.
		case breached && s.Status == StatusPending:
			if now.Sub(s.Since) >= r.For() {
				s.Status, s.Since = StatusFiring, now
				event(StatusFiring, v.Labels, v.Value)
			}
		case breached:
			s.Since = now
			if r.For() == 0 {
				s.Status = StatusFiring
				event(StatusFiring, v.Labels, v.Value)
			} else {
				s.Status = StatusPending
			}
		case s.Status == StatusFiring:
			s.Status, s.Since = StatusOK, now
			event(StatusResolved, v.Labels, v.Value)
		case s.Status == StatusPending:
			s.Status, s.Since = StatusOK, now
		}
		states = append(states, s)
	}

	// Groups that no longer report data stop firing.
	for g, p := range prev {
		if !seen[g] && p.Status == StatusFiring {
			event(StatusResolved, p.Labels, p.Value)
		}
	}
	slices.SortFunc(events, func(a, b Event) int { return strings.Compare(groupKey(a.Labels), groupKey(b.Labels)) })
	return states, events
}
