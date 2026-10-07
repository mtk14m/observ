// Package auth authenticates users of the web UI and the API: local
// accounts with bcrypt-hashed passwords and cookie sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials is returned for a wrong email or password. The
	// same error covers both so that it does not reveal who has an account.
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	// ErrNoSession is returned for missing, unknown or expired sessions.
	ErrNoSession = errors.New("auth: no valid session")
	// ErrInvalid wraps validation errors.
	ErrInvalid = errors.New("auth: invalid")
	// ErrExists is returned when an email is already taken.
	ErrExists = errors.New("auth: a user with this email already exists")
	// ErrNotFound is returned for unknown users.
	ErrNotFound = errors.New("auth: user not found")
)

// Roles.
const (
	// RoleAdmin can manage users.
	RoleAdmin = "admin"
	// RoleMember can use everything else.
	RoleMember = "member"
)

// MinPasswordLength is the minimum length of a password.
const MinPasswordLength = 10

// Migrations creates the authentication tables. Append only.
var Migrations = []string{
	`CREATE TABLE users (
	   id TEXT PRIMARY KEY,
	   email TEXT NOT NULL UNIQUE,
	   name TEXT NOT NULL,
	   role TEXT NOT NULL,
	   password_hash TEXT NOT NULL,
	   created_at INTEGER NOT NULL);
	 CREATE TABLE sessions (
	   token_hash TEXT PRIMARY KEY,
	   user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	   expires_at INTEGER NOT NULL);
	 CREATE INDEX sessions_user ON sessions (user_id);`,
}

// User is an account.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUser describes an account to create.
type NewUser struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// StoreOptions configures a Store.
type StoreOptions struct {
	Now func() time.Time
	// SessionTTL defaults to 30 days.
	SessionTTL time.Duration
	// FastHashing uses the cheapest bcrypt cost. For tests only.
	FastHashing bool
}

// Store persists users and sessions in SQLite.
type Store struct {
	db   *sql.DB
	opts StoreOptions
	cost int
	// dummyHash is compared against when an email is unknown, so that a
	// login attempt takes the same time whether or not the account exists.
	dummyHash []byte
}

// NewStore returns a store on a database migrated with Migrations.
func NewStore(db *sql.DB, opts StoreOptions) *Store {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 30 * 24 * time.Hour
	}
	cost := 12
	if opts.FastHashing {
		cost = bcrypt.MinCost
	}
	dummy, _ := bcrypt.GenerateFromPassword([]byte("obsrv-timing-equaliser"), cost)
	return &Store{db: db, opts: opts, cost: cost, dummyHash: dummy}
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// CreateUser validates and creates an account.
func (s *Store) CreateUser(ctx context.Context, n NewUser) (User, error) {
	email := normalizeEmail(n.Email)
	var problems []string
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		problems = append(problems, "a valid email is required")
	}
	if strings.TrimSpace(n.Name) == "" {
		problems = append(problems, "name is required")
	}
	if len(n.Password) < MinPasswordLength {
		problems = append(problems, fmt.Sprintf("password must be at least %d characters", MinPasswordLength))
	}
	if n.Role != RoleAdmin && n.Role != RoleMember {
		problems = append(problems, `role must be "admin" or "member"`)
	}
	if len(problems) > 0 {
		return User{}, fmt.Errorf("%w: %s", ErrInvalid, strings.Join(problems, "; "))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(n.Password), s.cost)
	if err != nil {
		return User{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	u := User{ID: ulid.Make().String(), Email: email, Name: strings.TrimSpace(n.Name), Role: n.Role,
		CreatedAt: s.opts.Now().UTC().Truncate(time.Second)}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, password_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.Name, u.Role, string(hash), u.CreatedAt.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return User{}, ErrExists
		}
		return User{}, fmt.Errorf("auth: %w", err)
	}
	return u, nil
}

// Authenticate checks an email and password.
func (s *Store) Authenticate(ctx context.Context, email, password string) (User, error) {
	var (
		u    User
		hash string
		at   int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role, password_hash, created_at FROM users WHERE email = ?`,
		normalizeEmail(email)).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &hash, &at)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	u.CreatedAt = time.Unix(at, 0).UTC()
	return u, nil
}

// GetUser returns a user.
func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	var at int64
	err := s.db.QueryRowContext(ctx, `SELECT id, email, name, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: %w", err)
	}
	u.CreatedAt = time.Unix(at, 0).UTC()
	return u, nil
}

// ListUsers returns every user, sorted by email.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, name, role, created_at FROM users ORDER BY email`)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []User{}
	for rows.Next() {
		var u User
		var at int64
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &at); err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
		u.CreatedAt = time.Unix(at, 0).UTC()
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers returns the number of accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("auth: %w", err)
	}
	return n, nil
}

// DeleteUser deletes an account and its sessions.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPassword changes a password and ends the user's sessions.
func (s *Store) SetPassword(ctx context.Context, id, password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalid, MinPasswordLength)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

// CreateSession starts a session and returns its secret token. Only a hash
// of the token is stored, so a leaked database does not leak sessions.
func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expires := s.opts.Now().Add(s.opts.SessionTTL).Unix()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), userID, expires); err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	// Opportunistic cleanup of expired sessions.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, s.opts.Now().Unix())
	return token, nil
}

// SessionUser returns the user of a valid session.
func (s *Store) SessionUser(ctx context.Context, token string) (User, error) {
	var userID string
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`,
		hashToken(token)).Scan(&userID, &expires)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && s.opts.Now().Unix() >= expires) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: %w", err)
	}
	u, err := s.GetUser(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrNoSession
	}
	return u, err
}

// DeleteSession ends a session.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token)); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
