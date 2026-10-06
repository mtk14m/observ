package query_test

import (
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/query"
)

func TestServiceDetail(t *testing.T) {
	d, err := fixture(t).ServiceDetail(t.Context(), "frontend", window, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary.Name != "frontend" || d.Summary.Requests != 2 {
		t.Errorf("summary = %+v", d.Summary)
	}
	// Spans at +10m and +20m fall in two 10-minute buckets.
	if len(d.Timeline) != 2 || d.Timeline[0].T != base.Add(10*time.Minute).UnixNano() ||
		d.Timeline[0].Requests != 1 || d.Timeline[0].P95Ms <= 0 {
		t.Errorf("timeline = %+v", d.Timeline)
	}
	if len(d.Operations) != 2 || d.Operations[0].Name != "GET /checkout" && d.Operations[0].Name != "GET /home" {
		t.Errorf("operations = %+v", d.Operations)
	}
	if len(d.Calls) != 1 || d.Calls[0].Service != "payment" || d.Calls[0].Requests != 1 || d.Calls[0].Errors != 1 {
		t.Errorf("calls = %+v", d.Calls)
	}
	if len(d.CalledBy) != 0 {
		t.Errorf("called by = %+v", d.CalledBy)
	}

	pay, err := fixture(t).ServiceDetail(t.Context(), "payment", window, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(pay.CalledBy) != 1 || pay.CalledBy[0].Service != "frontend" {
		t.Errorf("payment called by = %+v", pay.CalledBy)
	}
	if pay.Timeline[0].Errors != 1 {
		t.Errorf("payment timeline = %+v", pay.Timeline)
	}
}

func TestServiceDetailOfUnknownService(t *testing.T) {
	d, err := fixture(t).ServiceDetail(t.Context(), "nope", window, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary.Requests != 0 || len(d.Timeline) != 0 {
		t.Errorf("detail = %+v", d)
	}
}

func TestServiceMap(t *testing.T) {
	edges, err := fixture(t).ServiceMap(t.Context(), window)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0] != (query.Edge{From: "frontend", To: "payment", Requests: 1, Errors: 1}) {
		t.Errorf("edges = %+v", edges)
	}
}
