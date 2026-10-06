package query

import (
	"math"
	"reflect"
	"testing"

	"github.com/mtk14n/obsrv/pkg/schema"
)

const sec = int64(1e9)

func pt(series int64, group string, t int64, v float64) rawPoint {
	return rawPoint{SeriesID: series, Group: group, Labels: map[string]string{"g": group}, Time: t * sec, Value: v}
}

func values(s []Series) map[string][]Point {
	out := map[string][]Point{}
	for _, x := range s {
		out[x.Labels["g"]] = x.Points
	}
	return out
}

func TestGaugeUsesLastValuePerSeriesThenAggregates(t *testing.T) {
	kind := metricKind{Type: schema.MetricGauge}
	points := []rawPoint{
		pt(1, "a", 1, 10), pt(1, "a", 5, 20), // last value in bucket [0,10) is 20
		pt(2, "a", 3, 40),
		pt(1, "a", 12, 7),
		pt(3, "b", 2, 1),
	}
	got := values(computeSeries(kind, points, 0, 20*sec, 10*sec, "avg"))
	want := map[string][]Point{
		"a": {{T: 0, V: 30}, {T: 10 * sec, V: 7}},
		"b": {{T: 0, V: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	sum := values(computeSeries(kind, points, 0, 20*sec, 10*sec, "sum"))
	if sum["a"][0].V != 60 {
		t.Errorf("sum of last values = %v, want 60", sum["a"][0].V)
	}
	maxV := values(computeSeries(kind, points, 0, 20*sec, 10*sec, "max"))
	if maxV["a"][0].V != 40 {
		t.Errorf("max = %v, want 40", maxV["a"][0].V)
	}
}

func TestCumulativeCounterRateHandlesResets(t *testing.T) {
	kind := metricKind{Type: schema.MetricSum, Temporality: schema.TemporalityCumulative, Monotonic: true}
	points := []rawPoint{
		pt(1, "a", 1, 100), // baseline, no increment
		pt(1, "a", 5, 150), // +50
		pt(1, "a", 11, 20), // reset: +20
		pt(1, "a", 15, 60), // +40
	}
	got := values(computeSeries(kind, points, 0, 20*sec, 10*sec, ""))
	want := []Point{{T: 0, V: 5}, {T: 10 * sec, V: 6}} // 50/10s, 60/10s
	if !reflect.DeepEqual(got["a"], want) {
		t.Errorf("got %v, want %v", got["a"], want)
	}
}

func TestDeltaCounterRateSumsAcrossSeries(t *testing.T) {
	kind := metricKind{Type: schema.MetricSum, Temporality: schema.TemporalityDelta, Monotonic: true}
	points := []rawPoint{pt(1, "a", 1, 30), pt(2, "a", 2, 70)}
	got := values(computeSeries(kind, points, 0, 10*sec, 10*sec, "sum"))
	if want := []Point{{T: 0, V: 10}}; !reflect.DeepEqual(got["a"], want) {
		t.Errorf("got %v, want %v", got["a"], want)
	}
}

func hist(series int64, t int64, count int64, sum float64, buckets ...int64) rawPoint {
	return rawPoint{SeriesID: series, Group: "a", Labels: map[string]string{"g": "a"}, Time: t * sec,
		Count: count, Sum: sum, Bounds: []float64{0.1, 0.5, 1}, Buckets: buckets}
}

func TestCumulativeHistogram(t *testing.T) {
	kind := metricKind{Type: schema.MetricHistogram, Temporality: schema.TemporalityCumulative}
	points := []rawPoint{
		hist(1, 1, 10, 1, 10, 0, 0, 0),      // baseline
		hist(1, 5, 110, 31, 10, 50, 40, 10), // +100 observations, +30 sum
	}
	tests := map[string]float64{
		"count": 10,  // 100 observations / 10s
		"avg":   0.3, // 30 / 100
		"p50":   0.5, // rank 50 falls at the end of (0.1, 0.5]
		"p90":   1,   // rank 90 at the end of (0.5, 1]
		"p99":   1,   // overflow bucket clamps to the last bound
	}
	for agg, want := range tests {
		got := computeSeries(kind, points, 0, 10*sec, 10*sec, agg)
		if len(got) != 1 || len(got[0].Points) != 1 || math.Abs(got[0].Points[0].V-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", agg, got, want)
		}
	}
}

func TestQuantileInterpolatesInsideBuckets(t *testing.T) {
	bounds := []float64{1, 2}
	buckets := []float64{0, 10, 0} // all observations in (1, 2]
	if got := quantile(0.5, bounds, buckets); got != 1.5 {
		t.Errorf("p50 = %v, want 1.5", got)
	}
	if got := quantile(0.5, bounds, []float64{0, 0, 0}); !math.IsNaN(got) {
		t.Errorf("quantile of empty histogram = %v, want NaN", got)
	}
}

func TestPointsOutsideTheRangeAreIgnored(t *testing.T) {
	kind := metricKind{Type: schema.MetricGauge}
	got := computeSeries(kind, []rawPoint{pt(1, "a", 25, 1)}, 0, 20*sec, 10*sec, "avg")
	if len(got) != 0 {
		t.Errorf("got %v, want no series", got)
	}
}

func TestBucketsAreAlignedOnFrom(t *testing.T) {
	kind := metricKind{Type: schema.MetricGauge}
	got := computeSeries(kind, []rawPoint{pt(1, "a", 107, 3)}, 100*sec, 120*sec, 5*sec, "avg")
	if want := []Point{{T: 105 * sec, V: 3}}; len(got) != 1 || !reflect.DeepEqual(got[0].Points, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
