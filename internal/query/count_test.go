package query_test

import (
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/query"
)

func TestCountLogs(t *testing.T) {
	e := fixture(t)
	total, err := e.CountLogs(t.Context(), query.LogQuery{TimeRange: window}, nil)
	if err != nil || len(total) != 1 || total[0].Count != 2 || len(total[0].Labels) != 0 {
		t.Errorf("total = %+v, %v", total, err)
	}

	byService, err := e.CountLogs(t.Context(), query.LogQuery{TimeRange: window}, []string{"service.name"})
	if err != nil || len(byService) != 2 ||
		byService[0].Labels["service.name"] != "frontend" || byService[0].Count != 1 {
		t.Errorf("by service = %+v, %v", byService, err)
	}

	// No match still yields a zero count, so "fewer than N" rules can fire.
	none, err := e.CountLogs(t.Context(), query.LogQuery{TimeRange: query.TimeRange{From: base, To: base.Add(time.Minute)}}, nil)
	if err != nil || len(none) != 1 || none[0].Count != 0 {
		t.Errorf("none = %+v, %v", none, err)
	}
}

func TestCountLogsByLevel(t *testing.T) {
	got, err := fixture(t).CountLogs(t.Context(), query.LogQuery{TimeRange: window}, []string{"level"})
	if err != nil || len(got) != 2 || got[0].Labels["level"] != "ERROR" || got[1].Labels["level"] != "INFO" {
		t.Errorf("by level = %+v, %v", got, err)
	}
}

func TestLogFacet(t *testing.T) {
	got, err := fixture(t).LogFacet(t.Context(), query.LogQuery{TimeRange: window}, "service.name", 10)
	if err != nil || len(got) != 2 || got[0].Count < got[1].Count {
		t.Errorf("facet = %+v, %v; want values sorted by count", got, err)
	}
	limited, _ := fixture(t).LogFacet(t.Context(), query.LogQuery{TimeRange: window}, "service.name", 1)
	if len(limited) != 1 {
		t.Errorf("limited facet = %+v", limited)
	}
}
