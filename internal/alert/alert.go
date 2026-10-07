// Package alert evaluates alert rules on telemetry and notifies channels
// when a condition starts or stops holding.
package alert

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mtk14n/obsrv/internal/query/logsearch"
)

var (
	// ErrNotFound is returned for unknown rules or channels.
	ErrNotFound = errors.New("alert: not found")
	// ErrInvalid wraps validation errors.
	ErrInvalid = errors.New("alert: invalid")
	// ErrInUse is returned when deleting a channel that rules still use.
	ErrInUse = errors.New("alert: in use")
)

// Rule kinds.
const (
	// KindMetric compares an aggregated metric with the threshold.
	KindMetric = "metric"
	// KindLogs compares the number of matching log records with the threshold.
	KindLogs = "logs"
)

// Rule describes a condition to watch.
type Rule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`

	// Metric rules.
	Metric  string            `json:"metric,omitempty"`
	Agg     string            `json:"agg,omitempty"`
	Filters map[string]string `json:"filters,omitempty"`
	// Logs rules: a log search query (see package logsearch).
	Query string `json:"query,omitempty"`
	// GroupBy evaluates the condition separately for each value of these
	// attributes, e.g. one alert per service.
	GroupBy []string `json:"group_by,omitempty"`

	// Op compares the value with Threshold: >, >=, < or <=.
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	// WindowSeconds is the period the value is computed over.
	WindowSeconds int `json:"window_seconds"`
	// ForSeconds is how long the condition must hold before firing.
	ForSeconds int `json:"for_seconds"`

	Channels  []string  `json:"channels"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Window returns the evaluation window.
func (r Rule) Window() time.Duration { return time.Duration(r.WindowSeconds) * time.Second }

// For returns how long the condition must hold before firing.
func (r Rule) For() time.Duration { return time.Duration(r.ForSeconds) * time.Second }

// Breached reports whether value violates the rule's condition.
func (r Rule) Breached(value float64) bool {
	switch r.Op {
	case ">":
		return value > r.Threshold
	case ">=":
		return value >= r.Threshold
	case "<":
		return value < r.Threshold
	case "<=":
		return value <= r.Threshold
	default:
		return false
	}
}

// Validate checks everything that does not need the database.
func (r Rule) Validate() error {
	var problems []string
	if strings.TrimSpace(r.Name) == "" {
		problems = append(problems, "name is required")
	}
	switch r.Kind {
	case KindMetric:
		if r.Metric == "" {
			problems = append(problems, "metric is required")
		}
	case KindLogs:
		if _, err := logsearch.Compile(r.Query); err != nil {
			problems = append(problems, "query: "+err.Error())
		}
	default:
		problems = append(problems, `kind must be "metric" or "logs"`)
	}
	switch r.Op {
	case ">", ">=", "<", "<=":
	default:
		problems = append(problems, "op must be one of >, >=, <, <=")
	}
	if r.WindowSeconds < 30 {
		problems = append(problems, "window_seconds must be at least 30")
	}
	if r.ForSeconds < 0 {
		problems = append(problems, "for_seconds must not be negative")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(problems, "; "))
	}
	return nil
}

// Channel types.
const (
	ChannelWebhook = "webhook"
	ChannelSlack   = "slack"
)

// Channel is where notifications are sent.
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Validate checks the channel.
func (c Channel) Validate() error {
	var problems []string
	if strings.TrimSpace(c.Name) == "" {
		problems = append(problems, "name is required")
	}
	if c.Type != ChannelWebhook && c.Type != ChannelSlack {
		problems = append(problems, `type must be "webhook" or "slack"`)
	}
	if u, err := url.Parse(c.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		problems = append(problems, "url must be an http(s) URL")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(problems, "; "))
	}
	return nil
}

// Alert statuses.
const (
	StatusOK       = "ok"
	StatusPending  = "pending"
	StatusFiring   = "firing"
	StatusResolved = "resolved" // events only
)

// State is the current status of one group of a rule.
type State struct {
	RuleID string            `json:"rule_id"`
	Group  string            `json:"group"`
	Labels map[string]string `json:"labels"`
	Status string            `json:"status"`
	// Since is when the group entered its current status.
	Since time.Time `json:"since"`
	Value float64   `json:"value"`
}

// Event records a group starting or stopping to fire.
type Event struct {
	ID       int64             `json:"id"`
	RuleID   string            `json:"rule_id"`
	RuleName string            `json:"rule_name"`
	Labels   map[string]string `json:"labels"`
	Status   string            `json:"status"`
	Value    float64           `json:"value"`
	At       time.Time         `json:"at"`
}
