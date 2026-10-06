package parquet_test

import (
	"bytes"
	"reflect"
	"testing"

	pq "github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/format"

	"github.com/mtk14n/obsrv/internal/encoding/parquet"
	"github.com/mtk14n/obsrv/pkg/schema"
)

func roundTrip[T any](t *testing.T, rows []T) []T {
	t.Helper()
	var buf bytes.Buffer
	if err := parquet.Write(&buf, rows); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := parquet.Read[T](bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return got
}

func TestRoundTripLogs(t *testing.T) {
	rows := []schema.Log{
		{TimeUnixNano: 1, ServiceName: "a", SeverityText: "INFO", Body: "hello",
			Attributes: map[string]string{"k": "v"}, ResourceAttributes: map[string]string{"service.name": "a"}},
		{TimeUnixNano: 2, ServiceName: "b", Body: "", TraceID: "abc",
			Attributes: map[string]string{}, ResourceAttributes: map[string]string{}},
	}
	if got := roundTrip(t, rows); !reflect.DeepEqual(got, rows) {
		t.Errorf("got  %+v\nwant %+v", got, rows)
	}
}

func TestRoundTripSpansWithEvents(t *testing.T) {
	rows := []schema.Span{{
		TraceID: "t", SpanID: "s", Name: "GET /", Kind: "Server", DurationNano: 10, StatusCode: "Error",
		Attributes:         map[string]string{"http.route": "/"},
		ResourceAttributes: map[string]string{"service.name": "web"},
		Events: []schema.SpanEvent{
			{TimeUnixNano: 5, Name: "exception", Attributes: map[string]string{"exception.type": "E"}},
		},
	}}
	if got := roundTrip(t, rows); !reflect.DeepEqual(got, rows) {
		t.Errorf("got  %+v\nwant %+v", got, rows)
	}
}

func TestRoundTripMetricPoints(t *testing.T) {
	minV, maxV := 0.5, 9.0
	rows := []schema.MetricPoint{
		{MetricName: "g", Type: schema.MetricGauge, SeriesID: -42, Value: 1.5,
			Attributes: map[string]string{"a": "b"}, ResourceAttributes: map[string]string{"service.name": "x"}},
		{MetricName: "h", Type: schema.MetricHistogram, Count: 3, Sum: 4, Min: &minV, Max: &maxV,
			ExplicitBounds: []float64{1, 2}, BucketCounts: []int64{1, 1, 1},
			Attributes: map[string]string{"a": "b"}, ResourceAttributes: map[string]string{"service.name": "x"}},
	}
	got := roundTrip(t, rows)
	// Missing lists come back as empty lists, which is equivalent for readers.
	rows[0].ExplicitBounds, rows[0].BucketCounts = []float64{}, []int64{}
	if !reflect.DeepEqual(got, rows) {
		t.Errorf("got  %+v\nwant %+v", got, rows)
	}
}

func TestFilesAreZstdCompressed(t *testing.T) {
	var buf bytes.Buffer
	if err := parquet.Write(&buf, []schema.Log{{Body: "x"}}); err != nil {
		t.Fatal(err)
	}
	f, err := pq.OpenFile(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, rg := range f.Metadata().RowGroups {
		for _, col := range rg.Columns {
			if col.MetaData.Codec != format.Zstd {
				t.Errorf("column %v codec = %v, want ZSTD", col.MetaData.PathInSchema, col.MetaData.Codec)
			}
		}
	}
}
