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
		{"timeout", "contains(lower(body), ?)", []any{"timeout"}},
		{"Timeout", "contains(lower(body), ?)", []any{"timeout"}},
		{`"connection reset"`, "contains(lower(body), ?)", []any{"connection reset"}},
		{"service:api", "service_name = ?", []any{"api"}},
		{"level:error", "upper(severity_text) IN (?)", []any{"ERROR"}},
		{"level:warn,error", "upper(severity_text) IN (?, ?)", []any{"WARN", "ERROR"}},
		{"status:error", "upper(severity_text) IN (?)", []any{"ERROR"}},
		{"trace_id:abc123", "trace_id = ?", []any{"abc123"}},
		{"http.route:/pay", "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"http.route", "http.route", "/pay"}},
		{`user.name:"Ada L"`, "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"user.name", "user.name", "Ada L"}},
		{"url.full:http://x:8080/a", "coalesce(attributes[?], resource_attributes[?]) = ?", []any{"url.full", "url.full", "http://x:8080/a"}},
		{"-health", "NOT (contains(lower(body), ?))", []any{"health"}},
		{"-service:api", "NOT (service_name = ?)", []any{"api"}},
		{
			`service:api level:error "payment failed" -retry`,
			"service_name = ? AND upper(severity_text) IN (?) AND contains(lower(body), ?) AND NOT (contains(lower(body), ?))",
			[]any{"api", "ERROR", "payment failed", "retry"},
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
