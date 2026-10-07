package alert

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func rule(forSeconds int) Rule {
	return Rule{ID: "r1", Name: "High errors", Op: ">", Threshold: 5, WindowSeconds: 300, ForSeconds: forSeconds}
}

func val(group string, v float64) Value {
	if group == "" {
		return Value{Labels: map[string]string{}, Value: v}
	}
	return Value{Labels: map[string]string{"service": group}, Value: v}
}

func byGroup(states []State) map[string]State {
	m := map[string]State{}
	for _, s := range states {
		m[s.Group] = s
	}
	return m
}

func TestBreachFiresImmediatelyWithoutFor(t *testing.T) {
	states, events := transition(rule(0), nil, []Value{val("api", 9), val("web", 1)}, t0)
	got := byGroup(states)
	if got["service=api"].Status != StatusFiring || got["service=web"].Status != StatusOK {
		t.Errorf("states = %+v", states)
	}
	if len(events) != 1 || events[0].Status != StatusFiring || events[0].Labels["service"] != "api" ||
		events[0].Value != 9 || events[0].RuleName != "High errors" {
		t.Errorf("events = %+v", events)
	}
}

func TestForWaitsBeforeFiring(t *testing.T) {
	r := rule(120)
	states, events := transition(r, nil, []Value{val("", 9)}, t0)
	if states[0].Status != StatusPending || len(events) != 0 {
		t.Fatalf("first breach: %+v, %+v", states, events)
	}

	states, events = transition(r, byGroup(states), []Value{val("", 9)}, t0.Add(time.Minute))
	if states[0].Status != StatusPending || len(events) != 0 || !states[0].Since.Equal(t0) {
		t.Fatalf("after 1m: %+v, %+v", states, events)
	}

	states, events = transition(r, byGroup(states), []Value{val("", 9)}, t0.Add(2*time.Minute))
	if states[0].Status != StatusFiring || len(events) != 1 {
		t.Fatalf("after 2m: %+v, %+v", states, events)
	}
}

func TestPendingThatRecoversNeverFires(t *testing.T) {
	r := rule(120)
	states, _ := transition(r, nil, []Value{val("", 9)}, t0)
	states, events := transition(r, byGroup(states), []Value{val("", 1)}, t0.Add(time.Minute))
	if states[0].Status != StatusOK || len(events) != 0 {
		t.Errorf("recovered while pending: %+v, %+v", states, events)
	}
}

func TestFiringNotifiesOnceThenResolves(t *testing.T) {
	r := rule(0)
	states, _ := transition(r, nil, []Value{val("", 9)}, t0)
	states, events := transition(r, byGroup(states), []Value{val("", 12)}, t0.Add(time.Minute))
	if states[0].Status != StatusFiring || len(events) != 0 || states[0].Value != 12 || !states[0].Since.Equal(t0) {
		t.Fatalf("still firing: %+v, %+v", states, events)
	}
	states, events = transition(r, byGroup(states), []Value{val("", 2)}, t0.Add(2*time.Minute))
	if states[0].Status != StatusOK || len(events) != 1 || events[0].Status != StatusResolved || events[0].Value != 2 {
		t.Errorf("resolved: %+v, %+v", states, events)
	}
}

func TestDisappearedGroupsAreResolved(t *testing.T) {
	r := rule(0)
	states, _ := transition(r, nil, []Value{val("api", 9)}, t0)
	states, events := transition(r, byGroup(states), nil, t0.Add(time.Minute))
	if len(states) != 0 || len(events) != 1 || events[0].Status != StatusResolved || events[0].Labels["service"] != "api" {
		t.Errorf("states %+v, events %+v", states, events)
	}
}

func TestOperators(t *testing.T) {
	for _, tc := range []struct {
		op   string
		v    float64
		want bool
	}{
		{">", 6, true}, {">", 5, false}, {">=", 5, true}, {"<", 4, true}, {"<", 5, false}, {"<=", 5, true},
	} {
		r := Rule{Op: tc.op, Threshold: 5}
		if got := r.Breached(tc.v); got != tc.want {
			t.Errorf("%v %s 5 = %v, want %v", tc.v, tc.op, got, tc.want)
		}
	}
}
