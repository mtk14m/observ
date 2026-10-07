// Package notify delivers alert events to webhooks and Slack.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mtk14n/obsrv/internal/alert"
)

// Options configures a Sender.
type Options struct {
	// BaseURL is obsrv's public URL, used for links in notifications.
	BaseURL string
	Client  *http.Client
}

// Sender sends notifications.
type Sender struct {
	baseURL string
	client  *http.Client
}

// New returns a Sender. The default client times out after 10 seconds.
func New(opts Options) *Sender {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Sender{baseURL: strings.TrimSuffix(opts.BaseURL, "/"), client: opts.Client}
}

// WebhookPayload is the JSON document posted to webhook channels. It is a
// public contract: add fields, never rename or remove them.
type WebhookPayload struct {
	Status        string            `json:"status"`
	RuleID        string            `json:"rule_id"`
	RuleName      string            `json:"rule_name"`
	Labels        map[string]string `json:"labels"`
	Value         float64           `json:"value"`
	Op            string            `json:"op"`
	Threshold     float64           `json:"threshold"`
	WindowSeconds int               `json:"window_seconds"`
	At            time.Time         `json:"at"`
	URL           string            `json:"url"`
}

// Send delivers one event of rule to a channel.
func (s *Sender) Send(ctx context.Context, ch alert.Channel, rule alert.Rule, ev alert.Event) error {
	link := s.baseURL + "/alerts?rule=" + url.QueryEscape(rule.ID)
	var body any
	switch ch.Type {
	case alert.ChannelSlack:
		body = map[string]string{"text": slackText(rule, ev, link)}
	default:
		labels := ev.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		body = WebhookPayload{
			Status: ev.Status, RuleID: rule.ID, RuleName: rule.Name, Labels: labels, Value: ev.Value,
			Op: rule.Op, Threshold: rule.Threshold, WindowSeconds: rule.WindowSeconds, At: ev.At.UTC(), URL: link,
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep "<" and ">" readable in receivers' logs
	if err := enc.Encode(body); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ch.URL, &buf)
	if err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "obsrv")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("notify %q: %w", ch.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify %q: %s", ch.Name, resp.Status)
	}
	return nil
}

func slackText(rule alert.Rule, ev alert.Event, link string) string {
	icon, status := ":red_circle:", "FIRING"
	if ev.Status == alert.StatusResolved {
		icon, status = ":large_green_circle:", "RESOLVED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s *%s* · %s", icon, status, rule.Name)
	if len(ev.Labels) > 0 {
		pairs := make([]string, 0, len(ev.Labels))
		for k, v := range ev.Labels {
			pairs = append(pairs, k+"="+v)
		}
		slices.Sort(pairs)
		fmt.Fprintf(&b, " · `%s`", strings.Join(pairs, " "))
	}
	fmt.Fprintf(&b, "\n%s %s %s over the last %s · <%s|Open in obsrv>",
		num(ev.Value), rule.Op, num(rule.Threshold), shortDuration(rule.Window()), link)
	return b.String()
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func shortDuration(d time.Duration) string {
	s := d.String()
	s = strings.TrimSuffix(s, "0s")
	return strings.TrimSuffix(s, "0m")
}
