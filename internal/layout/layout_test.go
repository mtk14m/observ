package layout_test

import (
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/layout"
)

func TestPartitionAndKey(t *testing.T) {
	at := time.Date(2026, 10, 7, 13, 58, 12, 0, time.UTC)
	if got := layout.Partition(at); got != "date=2026-10-07/hour=13" {
		t.Errorf("Partition = %q", got)
	}
	if got := layout.Prefix(layout.Logs); got != "v1/logs/" {
		t.Errorf("Prefix = %q", got)
	}
	key := layout.NewKey(layout.Spans, at)
	if want := "v1/spans/date=2026-10-07/hour=13/"; len(key) <= len(want) || key[:len(want)] != want {
		t.Errorf("NewKey = %q, want prefix %q", key, want)
	}
	if layout.NewKey(layout.Spans, at) == key {
		t.Error("NewKey must be unique")
	}
}

func TestParseKey(t *testing.T) {
	k, ok := layout.ParseKey("v1/metric_points/date=2026-10-07/hour=09/01JA.parquet")
	if !ok || k.Dir != layout.MetricPoints || !k.Hour.Equal(time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)) ||
		k.Partition != "v1/metric_points/date=2026-10-07/hour=09/" {
		t.Errorf("ParseKey = %+v, %v", k, ok)
	}
	for _, bad := range []string{"v1/logs/x.parquet", "v1/logs/date=2026-13-01/hour=00/a.parquet", "v1/_compaction/a.json", "other"} {
		if _, ok := layout.ParseKey(bad); ok {
			t.Errorf("ParseKey(%q) succeeded", bad)
		}
	}
}
