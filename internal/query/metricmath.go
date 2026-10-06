package query

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/mtk14n/obsrv/pkg/schema"
)

// Point is one value of a time series. T is in unix nanoseconds.
type Point struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// Series is a time series identified by its group-by labels.
type Series struct {
	Labels map[string]string `json:"labels"`
	Points []Point           `json:"points"`
}

type metricKind struct {
	Type        string
	Temporality string
	Monotonic   bool
}

func (k metricKind) isCounter() bool { return k.Type == schema.MetricSum && k.Monotonic }

func (k metricKind) isDistribution() bool {
	return k.Type == schema.MetricHistogram || k.Type == schema.MetricExponentialHistogram ||
		k.Type == schema.MetricSummary
}

// rawPoint is a stored data point with its group-by key. Points must be
// sorted by series then time.
type rawPoint struct {
	SeriesID int64
	Group    string
	Labels   map[string]string
	Start    int64
	Time     int64
	Value    float64
	Count    int64
	Sum      float64
	Bounds   []float64
	Buckets  []int64
}

// seriesBucket accumulates one series within one time bucket.
type seriesBucket struct {
	group   string
	lastT   int64
	last    float64 // gauges
	inc     float64 // counters
	count   float64 // distributions
	sum     float64
	buckets []float64
	bounds  []float64
}

// computeSeries turns raw points into one series per group, with one point
// per step-sized bucket of [from, to).
//
//   - Gauges (and non-monotonic sums): the last value of each series in the
//     bucket, aggregated across series with agg (avg by default).
//   - Monotonic sums: the per-second rate of each series, handling counter
//     resets, aggregated across series with agg (sum by default).
//   - Histograms: observations of all series are merged, then agg selects
//     "count" (per second), "avg", or a percentile such as "p95".
func computeSeries(kind metricKind, points []rawPoint, from, to, step int64, agg string) []Series {
	type key struct {
		series int64
		bucket int64
	}
	acc := map[key]*seriesBucket{}
	labels := map[string]map[string]string{}
	prev := map[int64]rawPoint{}
	cumulative := kind.Temporality != schema.TemporalityDelta

	for _, p := range points {
		before, hasPrev := prev[p.SeriesID]
		prev[p.SeriesID] = p
		if p.Time < from || p.Time >= to {
			continue
		}
		k := key{p.SeriesID, (p.Time - from) / step}
		sb := acc[k]
		if sb == nil {
			sb = &seriesBucket{group: p.Group, lastT: math.MinInt64}
		}
		reset := !hasPrev || p.Start != before.Start

		switch {
		case kind.isCounter():
			switch {
			case !cumulative:
				sb.inc += p.Value
			case !hasPrev:
				continue // first point: baseline only
			case reset || p.Value < before.Value:
				sb.inc += p.Value
			default:
				sb.inc += p.Value - before.Value
			}
		case kind.isDistribution():
			count, sum, buckets := float64(p.Count), p.Sum, floats(p.Buckets)
			if cumulative && kind.Type != schema.MetricSummary {
				if !hasPrev {
					continue
				}
				if !reset && p.Count >= before.Count {
					count -= float64(before.Count)
					sum -= before.Sum
					if len(before.Buckets) == len(buckets) {
						for i := range buckets {
							buckets[i] -= float64(before.Buckets[i])
						}
					}
				}
			}
			sb.count += count
			sb.sum += sum
			sb.bounds = p.Bounds
			sb.buckets = addFloats(sb.buckets, buckets)
		default:
			if p.Time >= sb.lastT {
				sb.last, sb.lastT = p.Value, p.Time
			}
		}
		acc[k] = sb
		labels[p.Group] = p.Labels
	}

	// Group series buckets by (group, bucket).
	type gkey struct {
		group  string
		bucket int64
	}
	grouped := map[gkey][]*seriesBucket{}
	for k, sb := range acc {
		gk := gkey{sb.group, k.bucket}
		grouped[gk] = append(grouped[gk], sb)
	}

	stepSeconds := float64(step) / 1e9
	bySeries := map[string][]Point{}
	for gk, sbs := range grouped {
		v, ok := aggregateBucket(kind, sbs, agg, stepSeconds)
		if !ok {
			continue
		}
		bySeries[gk.group] = append(bySeries[gk.group], Point{T: from + gk.bucket*step, V: v})
	}

	out := make([]Series, 0, len(bySeries))
	for group, pts := range bySeries {
		slices.SortFunc(pts, func(a, b Point) int { return cmp.Compare(a.T, b.T) })
		out = append(out, Series{Labels: labels[group], Points: pts})
	}
	slices.SortFunc(out, func(a, b Series) int { return cmp.Compare(labelKey(a.Labels), labelKey(b.Labels)) })
	return out
}

func aggregateBucket(kind metricKind, sbs []*seriesBucket, agg string, stepSeconds float64) (float64, bool) {
	if kind.isDistribution() {
		var count, sum float64
		var buckets, bounds []float64
		for _, sb := range sbs {
			count += sb.count
			sum += sb.sum
			if len(sb.bounds) > 0 && (bounds == nil || slices.Equal(bounds, sb.bounds)) {
				bounds = sb.bounds
				buckets = addFloats(buckets, sb.buckets)
			}
		}
		switch {
		case agg == "count":
			return count / stepSeconds, true
		case agg == "" || agg == "avg":
			if count == 0 {
				return 0, false
			}
			return sum / count, true
		case strings.HasPrefix(agg, "p"):
			q, err := strconv.ParseFloat(agg[1:], 64)
			if err != nil || q <= 0 || q >= 100 {
				return 0, false
			}
			v := quantile(q/100, bounds, buckets)
			return v, !math.IsNaN(v)
		default:
			return 0, false
		}
	}

	vals := make([]float64, 0, len(sbs))
	for _, sb := range sbs {
		if kind.isCounter() {
			vals = append(vals, sb.inc/stepSeconds)
		} else {
			vals = append(vals, sb.last)
		}
	}
	if agg == "" {
		agg = "avg"
		if kind.isCounter() {
			agg = "sum"
		}
	}
	return reduce(agg, vals)
}

func reduce(agg string, vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	switch agg {
	case "sum":
		var s float64
		for _, v := range vals {
			s += v
		}
		return s, true
	case "avg":
		v, _ := reduce("sum", vals)
		return v / float64(len(vals)), true
	case "min":
		return slices.Min(vals), true
	case "max":
		return slices.Max(vals), true
	case "count":
		return float64(len(vals)), true
	default:
		return 0, false
	}
}

// quantile estimates the q-quantile (0 < q < 1) of an explicit-bucket
// histogram by linear interpolation inside the bucket holding the rank.
// buckets has len(bounds)+1 entries; values in the overflow bucket are
// clamped to the last bound. It returns NaN for an empty histogram.
func quantile(q float64, bounds, buckets []float64) float64 {
	var total float64
	for _, c := range buckets {
		total += c
	}
	if total <= 0 || len(bounds) == 0 || len(buckets) != len(bounds)+1 {
		return math.NaN()
	}
	rank := q * total
	var cum float64
	for i, c := range buckets {
		if c <= 0 || cum+c < rank {
			cum += c
			continue
		}
		if i == len(bounds) {
			return bounds[len(bounds)-1]
		}
		lower := 0.0
		if i > 0 {
			lower = bounds[i-1]
		} else if bounds[0] <= 0 {
			return bounds[0]
		}
		return lower + (bounds[i]-lower)*((rank-cum)/c)
	}
	return bounds[len(bounds)-1]
}

func floats(in []int64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

func addFloats(acc, add []float64) []float64 {
	if len(acc) == 0 {
		return slices.Clone(add)
	}
	if len(acc) != len(add) {
		return acc
	}
	for i := range add {
		acc[i] += add[i]
	}
	return acc
}

func labelKey(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(0)
		b.WriteString(m[k])
		b.WriteByte(0)
	}
	return b.String()
}
