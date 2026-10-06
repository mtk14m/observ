package main

import (
	"testing"
	"time"
)

func TestGeneratorProducesTheRequestedVolume(t *testing.T) {
	g := newGenerator(1, 5, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if n := g.logs(100, 0, 100, time.Hour).LogRecordCount(); n != 100 {
		t.Errorf("logs = %d, want 100", n)
	}
	if n := g.traces(100, 0, 100, time.Hour).SpanCount(); n < 100 || n > 106 {
		t.Errorf("spans = %d, want about 100", n)
	}
	// 5 services × (1 gauge + 7 routes × (counter + histogram)).
	if n := g.metrics(0, time.Now()).DataPointCount(); n != 5*(1+7*2) {
		t.Errorf("metric points = %d, want %d", n, 5*15)
	}
}
