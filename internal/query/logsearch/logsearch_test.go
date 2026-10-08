package logsearch_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		query string
		where string
		args  []any
	}{
		{"", "TRUE", nil},
		{"   ", "TRUE", nil},
		{"timeout", "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))", []any{"timeout", "timeout"}},
		{"Timeout", "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))", []any{"timeout", "timeout"}},
		{`"connection reset"`, "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))", []any{"connection reset", "connection reset"}},
		{"service:api", "service_name = ?", []any{"api"}},
		{"level:error", "upper(severity_text) IN (?)", []any{"ERROR"}},
		{"level:warn,error", "upper(severity_text) IN (?, ?)", []any{"WARN", "ERROR"}},
		{"status:error", "upper(severity_text) IN (?)", []any{"ERROR"}},
		{"trace_id:abc123", "trace_id = ?", []any{"abc123"}},
		{"http.route:/pay", "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"http.route", "http.route", "/pay"}},
		{`user.name:"Ada L"`, "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"user.name", "user.name", "Ada L"}},
		{"url.full:http://x:8080/a", "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"url.full", "url.full", "http://x:8080/a"}},
		{"-health", "NOT (" + "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))" + ")", []any{"health", "health"}},
		{"-service:api", "NOT (service_name = ?)", []any{"api"}},
		{
			`service:api level:error "payment failed" -retry`,
			"service_name = ? AND upper(severity_text) IN (?) AND " + "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))" + " AND NOT (" + "(contains(lower(body), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))" + ")",
			[]any{"api", "ERROR", "payment failed", "payment failed", "retry", "retry"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := logsearch.Compile(tt.query)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got.Where != tt.where {
				t.Errorf("Where = %q, want %q", got.Where, tt.where)
			}
			if !reflect.DeepEqual(got.Args, tt.args) {
				t.Errorf("Args = %#v, want %#v", got.Args, tt.args)
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	for _, q := range []string{`"unterminated`, "service:", ":value", "-"} {
		t.Run(q, func(t *testing.T) {
			if _, err := logsearch.Compile(q); !errors.Is(err, logsearch.ErrSyntax) {
				t.Errorf("Compile(%q) error = %v, want ErrSyntax", q, err)
			}
		})
	}
}

func TestCompileSpans(t *testing.T) {
	tests := []struct {
		query string
		where string
		args  []any
	}{
		{"checkout", "(contains(lower(name), ?) OR contains(lower(array_to_string(map_values(attributes), ' ')), ?))", []any{"checkout", "checkout"}},
		{"status:error", "status_code IN (?)", []any{"Error"}},
		{"status:ok,unset", "status_code IN (?, ?)", []any{"Ok", "Unset"}},
		{"service:payment", "service_name = ?", []any{"payment"}},
		{"payment.issuer:acme-bank", "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"payment.issuer", "payment.issuer", "acme-bank"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := logsearch.CompileSpans(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got.Where != tt.where || !reflect.DeepEqual(got.Args, tt.args) {
				t.Errorf("got %q %#v, want %q %#v", got.Where, got.Args, tt.where, tt.args)
			}
		})
	}
	if _, err := logsearch.CompileSpans("status:broken"); !errors.Is(err, logsearch.ErrSyntax) {
		t.Errorf("unknown status: %v, want ErrSyntax", err)
	}
}
