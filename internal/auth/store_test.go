package auth_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mtk14n/obsrv/internal/auth"
	"github.com/mtk14n/obsrv/internal/metadata"
)

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *auth.Store {
	t.Helper()
	db, err := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"))
	if err == nil {
		err = metadata.Migrate(t.Context(), db, "auth", auth.Migrations)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return auth.NewStore(db, auth.StoreOptions{Now: func() time.Time { return now }, FastHashing: true})
}

func TestCreateAndAuthenticate(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	u, err := s.CreateUser(ctx, auth.NewUser{Email: "Ada@Example.com ", Name: "Ada", Password: "correct horse battery", Role: auth.RoleAdmin})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != "ada@example.com" || u.ID == "" || u.Role != auth.RoleAdmin {
		t.Errorf("user = %+v", u)
	}

	got, err := s.Authenticate(ctx, "ADA@example.com", "correct horse battery")
	if err != nil || got.ID != u.ID {
		t.Errorf("Authenticate = %+v, %v", got, err)
	}
	for _, tc := range []struct{ email, password string }{
		{"ada@example.com", "wrong password!!"},
		{"nobody@example.com", "correct horse battery"},
	} {
		if _, err := s.Authenticate(ctx, tc.email, tc.password); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("Authenticate(%s) = %v, want ErrInvalidCredentials", tc.email, err)
		}
	}
}

func TestPasswordsAreHashed(t *testing.T) {
	s := newStore(t)
	_, _ = s.CreateUser(t.Context(), auth.NewUser{Email: "a@b.c", Name: "A", Password: "a secret password", Role: auth.RoleMember})
	hash, err := s.PasswordHashForTest(t.Context(), "a@b.c")
	if err != nil || hash == "" || strings.Contains(hash, "secret") || !strings.HasPrefix(hash, "$2") {
		t.Errorf("stored hash = %q, %v; want a bcrypt hash", hash, err)
	}
}

func TestValidation(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	cases := map[string]auth.NewUser{
		"bad email":      {Email: "not-an-email", Name: "x", Password: "long enough pw", Role: auth.RoleMember},
		"short password": {Email: "a@b.c", Name: "x", Password: "short", Role: auth.RoleMember},
		"unknown role":   {Email: "a@b.c", Name: "x", Password: "long enough pw", Role: "god"},
	}
	for name, u := range cases {
		if _, err := s.CreateUser(ctx, u); !errors.Is(err, auth.ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	_, _ = s.CreateUser(ctx, auth.NewUser{Email: "a@b.c", Name: "x", Password: "long enough pw", Role: auth.RoleMember})
	if _, err := s.CreateUser(ctx, auth.NewUser{Email: "A@b.c", Name: "y", Password: "long enough pw", Role: auth.RoleMember}); !errors.Is(err, auth.ErrExists) {
		t.Errorf("duplicate email: %v, want ErrExists", err)
	}
}

func TestSessions(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	u, _ := s.CreateUser(ctx, auth.NewUser{Email: "a@b.c", Name: "A", Password: "long enough pw", Role: auth.RoleMember})

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil || len(token) < 40 {
		t.Fatalf("CreateSession = %q, %v", token, err)
	}
	got, err := s.SessionUser(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Errorf("SessionUser = %+v, %v", got, err)
	}
	if _, err := s.SessionUser(ctx, "forged-token"); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("forged token: %v, want ErrNoSession", err)
	}

	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("after logout: %v, want ErrNoSession", err)
	}
}

func TestSessionsExpire(t *testing.T) {
	db, _ := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"))
	_ = metadata.Migrate(t.Context(), db, "auth", auth.Migrations)
	defer func() { _ = db.Close() }()
	clock := now
	s := auth.NewStore(db, auth.StoreOptions{Now: func() time.Time { return clock }, SessionTTL: time.Hour, FastHashing: true})
	u, _ := s.CreateUser(t.Context(), auth.NewUser{Email: "a@b.c", Name: "A", Password: "long enough pw", Role: auth.RoleMember})
	token, _ := s.CreateSession(t.Context(), u.ID)

	clock = now.Add(2 * time.Hour)
	if _, err := s.SessionUser(t.Context(), token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("expired session: %v, want ErrNoSession", err)
	}
}

func TestPasswordChangeAndDeletionEndSessions(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	u, _ := s.CreateUser(ctx, auth.NewUser{Email: "a@b.c", Name: "A", Password: "long enough pw", Role: auth.RoleMember})
	token, _ := s.CreateSession(ctx, u.ID)

	if err := s.SetPassword(ctx, u.ID, "a brand new password"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("sessions must end when the password changes: %v", err)
	}
	if _, err := s.Authenticate(ctx, "a@b.c", "a brand new password"); err != nil {
		t.Errorf("new password: %v", err)
	}

	token, _ = s.CreateSession(ctx, u.ID)
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("sessions of a deleted user must end: %v", err)
	}
}

func TestListAndCount(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Errorf("CountUsers = %d", n)
	}
	_, _ = s.CreateUser(ctx, auth.NewUser{Email: "b@x.io", Name: "B", Password: "long enough pw", Role: auth.RoleMember})
	_, _ = s.CreateUser(ctx, auth.NewUser{Email: "a@x.io", Name: "A", Password: "long enough pw", Role: auth.RoleAdmin})
	users, err := s.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[0].Email != "a@x.io" {
		t.Errorf("ListUsers = %+v, %v", users, err)
	}
	if n, _ := s.CountUsers(ctx); n != 2 {
		t.Errorf("CountUsers = %d", n)
	}
}
