// Package logsearch compiles the log search syntax into a SQL predicate.
//
// The syntax is a list of terms, all of which must match:
//
//	timeout               the body contains "timeout" (case-insensitive)
//	"connection reset"    the body contains the phrase
//	service:api           the service is "api"
//	level:warn,error      the severity is WARN or ERROR (also status:, severity:)
//	trace_id:4bf92f…      the record belongs to the trace
//	http.route:/pay       an attribute (record or resource) equals the value
//	-term                 negates any term
package logsearch

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrSyntax is returned for malformed queries.
var ErrSyntax = errors.New("logsearch: syntax error")

// Predicate is a SQL boolean expression with positional (?) arguments.
type Predicate struct {
	Where string
	Args  []any
}

// Compile parses query into a predicate over the public logs schema.
func Compile(query string) (Predicate, error) {
	terms, err := tokenize(query)
	if err != nil {
		return Predicate{}, err
	}
	if len(terms) == 0 {
		return Predicate{Where: "TRUE"}, nil
	}
	var clauses []string
	var args []any
	for _, t := range terms {
		clause, a, err := compileTerm(t)
		if err != nil {
			return Predicate{}, err
		}
		if t.negated {
			clause = "NOT (" + clause + ")"
		}
		clauses = append(clauses, clause)
		args = append(args, a...)
	}
	return Predicate{Where: strings.Join(clauses, " AND "), Args: args}, nil
}

type term struct {
	negated bool
	key     string // empty for free text
	value   string
}

func compileTerm(t term) (string, []any, error) {
	switch strings.ToLower(t.key) {
	case "":
		return "contains(lower(body), ?)", []any{strings.ToLower(t.value)}, nil
	case "service":
		return "service_name = ?", []any{t.value}, nil
	case "level", "status", "severity":
		levels := strings.Split(t.value, ",")
		args := make([]any, 0, len(levels))
		for _, l := range levels {
			if l = strings.TrimSpace(l); l == "" {
				return "", nil, fmt.Errorf("%w: empty level in %q", ErrSyntax, t.value)
			}
			args = append(args, strings.ToUpper(l))
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(args)), ", ")
		return "upper(severity_text) IN (" + placeholders + ")", args, nil
	case "trace_id":
		return "trace_id = ?", []any{t.value}, nil
	default:
		return "coalesce(attributes[?], resource_attributes[?]) = ?", []any{t.key, t.key, t.value}, nil
	}
}

// tokenize splits the query into terms, honouring double quotes.
func tokenize(q string) ([]term, error) {
	var terms []term
	r := []rune(q)
	for i := 0; i < len(r); {
		if unicode.IsSpace(r[i]) {
			i++
			continue
		}
		var t term
		if r[i] == '-' {
			t.negated = true
			i++
		}
		word, next, quoted, err := readWord(r, i, true)
		if err != nil {
			return nil, err
		}
		i = next
		if !quoted && i < len(r) && r[i] == ':' { // key:value
			value, next, _, err := readWord(r, i+1, false)
			if err != nil {
				return nil, err
			}
			if word == "" || value == "" {
				return nil, fmt.Errorf("%w: expected key:value", ErrSyntax)
			}
			t.key, t.value, i = word, value, next
		} else {
			if word == "" {
				return nil, fmt.Errorf("%w: empty term", ErrSyntax)
			}
			t.value = word
		}
		terms = append(terms, t)
	}
	return terms, nil
}

// readWord reads a quoted string or a bare word. A bare word stops at a
// space, and also at ':' when stopAtColon is set (keys, not values).
func readWord(r []rune, i int, stopAtColon bool) (word string, next int, quoted bool, err error) {
	if i < len(r) && r[i] == '"' {
		end := i + 1
		for end < len(r) && r[end] != '"' {
			end++
		}
		if end >= len(r) {
			return "", 0, false, fmt.Errorf("%w: unterminated quote", ErrSyntax)
		}
		return string(r[i+1 : end]), end + 1, true, nil
	}
	start := i
	for i < len(r) && !unicode.IsSpace(r[i]) && (!stopAtColon || r[i] != ':') {
		i++
	}
	return string(r[start:i]), i, false, nil
}
