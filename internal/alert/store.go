package alert

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Migrations creates the alerting tables. Append only.
var Migrations = []string{
	`CREATE TABLE alert_channels (id TEXT PRIMARY KEY, data TEXT NOT NULL);
	 CREATE TABLE alert_rules (id TEXT PRIMARY KEY, data TEXT NOT NULL);
	 CREATE TABLE alert_states (
	   rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
	   group_key TEXT NOT NULL,
	   data TEXT NOT NULL,
	   PRIMARY KEY (rule_id, group_key));
	 CREATE TABLE alert_events (id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER NOT NULL, data TEXT NOT NULL);
	 CREATE INDEX alert_events_at ON alert_events (at);`,
}

// Store persists rules, channels, states and events in SQLite.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// NewStore returns a store on a database migrated with Migrations.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, now: func() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }}
}

// --- Rules -------------------------------------------------------------------

// CreateRule validates and saves a new rule.
func (s *Store) CreateRule(ctx context.Context, r Rule) (Rule, error) {
	if err := s.validateRule(ctx, r); err != nil {
		return Rule{}, err
	}
	r.ID = ulid.Make().String()
	r.CreatedAt = s.now()
	r.UpdatedAt = r.CreatedAt
	return r, s.put(ctx, "INSERT INTO alert_rules (id, data) VALUES (?, ?)", r.ID, r)
}

// UpdateRule replaces a rule, keeping its ID and creation time.
func (s *Store) UpdateRule(ctx context.Context, id string, r Rule) (Rule, error) {
	old, err := s.GetRule(ctx, id)
	if err != nil {
		return Rule{}, err
	}
	if err := s.validateRule(ctx, r); err != nil {
		return Rule{}, err
	}
	r.ID, r.CreatedAt, r.UpdatedAt = id, old.CreatedAt, s.now()
	return r, s.put(ctx, "UPDATE alert_rules SET data = ? WHERE id = ?", r, id)
}

// GetRule returns a rule.
func (s *Store) GetRule(ctx context.Context, id string) (Rule, error) {
	var r Rule
	return r, s.get(ctx, "SELECT data FROM alert_rules WHERE id = ?", id, &r)
}

// ListRules returns every rule, sorted by name.
func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rules, err := list[Rule](ctx, s.db, "SELECT data FROM alert_rules")
	slices.SortFunc(rules, func(a, b Rule) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return rules, err
}

// DeleteRule deletes a rule and its states.
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	return s.del(ctx, "DELETE FROM alert_rules WHERE id = ?", id)
}

func (s *Store) validateRule(ctx context.Context, r Rule) error {
	if err := r.Validate(); err != nil {
		return err
	}
	for _, id := range r.Channels {
		if _, err := s.GetChannel(ctx, id); errors.Is(err, ErrNotFound) {
			return fmt.Errorf("%w: unknown channel %q", ErrInvalid, id)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// --- Channels ----------------------------------------------------------------

// CreateChannel validates and saves a channel.
func (s *Store) CreateChannel(ctx context.Context, c Channel) (Channel, error) {
	if err := c.Validate(); err != nil {
		return Channel{}, err
	}
	c.ID = ulid.Make().String()
	return c, s.put(ctx, "INSERT INTO alert_channels (id, data) VALUES (?, ?)", c.ID, c)
}

// UpdateChannel replaces a channel.
func (s *Store) UpdateChannel(ctx context.Context, id string, c Channel) (Channel, error) {
	if _, err := s.GetChannel(ctx, id); err != nil {
		return Channel{}, err
	}
	if err := c.Validate(); err != nil {
		return Channel{}, err
	}
	c.ID = id
	return c, s.put(ctx, "UPDATE alert_channels SET data = ? WHERE id = ?", c, id)
}

// GetChannel returns a channel.
func (s *Store) GetChannel(ctx context.Context, id string) (Channel, error) {
	var c Channel
	return c, s.get(ctx, "SELECT data FROM alert_channels WHERE id = ?", id, &c)
}

// ListChannels returns every channel, sorted by name.
func (s *Store) ListChannels(ctx context.Context) ([]Channel, error) {
	chans, err := list[Channel](ctx, s.db, "SELECT data FROM alert_channels")
	slices.SortFunc(chans, func(a, b Channel) int { return strings.Compare(a.Name, b.Name) })
	return chans, err
}

// DeleteChannel deletes a channel no rule uses.
func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	rules, err := s.ListRules(ctx)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if slices.Contains(r.Channels, id) {
			return fmt.Errorf("%w: rule %q sends to this channel", ErrInUse, r.Name)
		}
	}
	return s.del(ctx, "DELETE FROM alert_channels WHERE id = ?", id)
}

// --- States and events -------------------------------------------------------

// States returns the state of every group of every rule.
func (s *Store) States(ctx context.Context) ([]State, error) {
	return list[State](ctx, s.db, "SELECT data FROM alert_states ORDER BY rule_id, group_key")
}

// ReplaceStates sets the states of a rule's groups, dropping the others.
func (s *Store) ReplaceStates(ctx context.Context, ruleID string, states []State) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("alert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM alert_states WHERE rule_id = ?", ruleID); err != nil {
		return fmt.Errorf("alert: %w", err)
	}
	for _, st := range states {
		b, _ := json.Marshal(st)
		if _, err := tx.ExecContext(ctx, "INSERT INTO alert_states (rule_id, group_key, data) VALUES (?, ?, ?)",
			ruleID, st.Group, string(b)); err != nil {
			return fmt.Errorf("alert: %w", err)
		}
	}
	return tx.Commit()
}

// AddEvents records alert events.
func (s *Store) AddEvents(ctx context.Context, events []Event) error {
	for _, e := range events {
		b, _ := json.Marshal(e)
		if _, err := s.db.ExecContext(ctx, "INSERT INTO alert_events (at, data) VALUES (?, ?)", e.At.UnixNano(), string(b)); err != nil {
			return fmt.Errorf("alert: %w", err)
		}
	}
	return nil
}

// Events returns the latest events, newest first.
func (s *Store) Events(ctx context.Context, limit int) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, data FROM alert_events ORDER BY at DESC, id DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("alert: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Event{}
	for rows.Next() {
		var id int64
		var data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("alert: %w", err)
		}
		var e Event
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			return nil, fmt.Errorf("alert: %w", err)
		}
		e.ID = id
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- helpers -----------------------------------------------------------------

// put runs an INSERT (id, data) or UPDATE (data, id) with v encoded as JSON.
func (s *Store) put(ctx context.Context, stmt string, a, b any) error {
	enc := func(v any) any {
		if str, ok := v.(string); ok {
			return str
		}
		data, _ := json.Marshal(v)
		return string(data)
	}
	if _, err := s.db.ExecContext(ctx, stmt, enc(a), enc(b)); err != nil {
		return fmt.Errorf("alert: %w", err)
	}
	return nil
}

func (s *Store) get(ctx context.Context, stmt, id string, v any) error {
	var data string
	err := s.db.QueryRowContext(ctx, stmt, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err != nil {
		return fmt.Errorf("alert: %w", err)
	}
	return json.Unmarshal([]byte(data), v)
}

func (s *Store) del(ctx context.Context, stmt, id string) error {
	res, err := s.db.ExecContext(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("alert: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return nil
}

func list[T any](ctx context.Context, db *sql.DB, stmt string) ([]T, error) {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("alert: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []T{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("alert: %w", err)
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, fmt.Errorf("alert: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
