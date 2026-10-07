package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtk14n/obsrv/internal/auth"
	"github.com/mtk14n/obsrv/internal/metadata"
)

type env struct {
	srv   *httptest.Server
	store *auth.Store
}

func newEnv(t *testing.T) env {
	t.Helper()
	db, err := metadata.Open(t.Context(), filepath.Join(t.TempDir(), "obsrv.db"))
	if err == nil {
		err = metadata.Migrate(t.Context(), db, "auth", auth.Migrations)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := auth.NewStore(db, auth.StoreOptions{FastHashing: true})
	h := auth.NewHandler(store, auth.HandlerOptions{})

	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		_, _ = io.WriteString(w, "hello "+u.Email)
	})
	mux := http.NewServeMux()
	h.Register(mux)
	mux.Handle("/api/", h.Require(protected))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return env{srv: srv, store: store}
}

// client is a browser-like client with its own cookie jar.
func (e env) client(t *testing.T) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func (e env) do(t *testing.T, c *http.Client, method, path string, body any, headers ...string) (int, string, http.Header) {
	t.Helper()
	var b io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		b = bytes.NewReader(data)
	}
	req, _ := http.NewRequestWithContext(t.Context(), method, e.srv.URL+path, b)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out), resp.Header
}

var admin = map[string]string{"email": "ada@example.com", "name": "Ada", "password": "correct horse battery"}

func TestSetupCreatesTheFirstAdminOnce(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	if _, body, _ := e.do(t, c, http.MethodGet, "/api/v1/auth/status", nil); !strings.Contains(body, `"setup_required":true`) {
		t.Fatalf("status = %s", body)
	}
	code, body, headers := e.do(t, c, http.MethodPost, "/api/v1/auth/setup", admin)
	if code != http.StatusCreated || !strings.Contains(body, `"role":"admin"`) {
		t.Fatalf("setup: %d %s", code, body)
	}
	cookie := headers.Get("Set-Cookie")
	for _, attr := range []string{"obsrv_session=", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(cookie, attr) {
			t.Errorf("cookie %q lacks %s", cookie, attr)
		}
	}
	// The setup session is usable right away.
	if code, body, _ := e.do(t, c, http.MethodGet, "/api/v1/data", nil); code != http.StatusOK || body != "hello ada@example.com" {
		t.Errorf("after setup: %d %s", code, body)
	}
	// A second setup is refused.
	if code, _, _ := e.do(t, e.client(t), http.MethodPost, "/api/v1/auth/setup", admin); code != http.StatusConflict {
		t.Errorf("second setup: %d, want 409", code)
	}
}

func TestAPIRequiresASession(t *testing.T) {
	e := newEnv(t)
	code, body, _ := e.do(t, e.client(t), http.MethodGet, "/api/v1/data", nil)
	if code != http.StatusUnauthorized || !strings.Contains(body, "error") {
		t.Errorf("anonymous: %d %s", code, body)
	}
}

func TestLoginAndLogout(t *testing.T) {
	e := newEnv(t)
	_, _ = e.store.CreateUser(t.Context(), auth.NewUser{Email: "ada@example.com", Name: "Ada", Password: "correct horse battery", Role: auth.RoleMember})
	c := e.client(t)

	code, body, _ := e.do(t, c, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "ada@example.com", "password": "nope nope nope"})
	if code != http.StatusUnauthorized || !strings.Contains(body, "invalid email or password") {
		t.Errorf("wrong password: %d %s", code, body)
	}
	code, _, _ = e.do(t, c, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "ada@example.com", "password": "correct horse battery"})
	if code != http.StatusOK {
		t.Fatalf("login: %d", code)
	}
	if _, body, _ := e.do(t, c, http.MethodGet, "/api/v1/auth/me", nil); !strings.Contains(body, `"email":"ada@example.com"`) {
		t.Errorf("me = %s", body)
	}
	if code, _, _ := e.do(t, c, http.MethodPost, "/api/v1/auth/logout", nil); code != http.StatusNoContent {
		t.Errorf("logout: %d", code)
	}
	if code, _, _ := e.do(t, c, http.MethodGet, "/api/v1/data", nil); code != http.StatusUnauthorized {
		t.Errorf("after logout: %d, want 401", code)
	}
}

func TestRepeatedFailuresAreThrottled(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	bad := map[string]string{"email": "ada@example.com", "password": "guess guess guess"}
	for range 5 {
		e.do(t, c, http.MethodPost, "/api/v1/auth/login", bad)
	}
	code, _, headers := e.do(t, c, http.MethodPost, "/api/v1/auth/login", bad)
	if code != http.StatusTooManyRequests || headers.Get("Retry-After") == "" {
		t.Errorf("sixth attempt: %d, Retry-After %q; want 429", code, headers.Get("Retry-After"))
	}
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	e := newEnv(t)
	code, _, _ := e.do(t, e.client(t), http.MethodPost, "/api/v1/auth/setup", admin, "Origin", "https://evil.example")
	if code != http.StatusForbidden {
		t.Errorf("cross-origin setup: %d, want 403", code)
	}
	code, _, _ = e.do(t, e.client(t), http.MethodPost, "/api/v1/auth/setup", admin, "Origin", e.srv.URL)
	if code != http.StatusCreated {
		t.Errorf("same-origin setup: %d, want 201", code)
	}
}

func TestUserManagementIsForAdmins(t *testing.T) {
	e := newEnv(t)
	ac := e.client(t)
	e.do(t, ac, http.MethodPost, "/api/v1/auth/setup", admin)

	code, body, _ := e.do(t, ac, http.MethodPost, "/api/v1/users",
		map[string]string{"email": "bob@example.com", "name": "Bob", "password": "bobs long password", "role": "member"})
	if code != http.StatusCreated {
		t.Fatalf("create user: %d %s", code, body)
	}
	var bob struct {
		Data auth.User `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &bob)

	if _, body, _ := e.do(t, ac, http.MethodGet, "/api/v1/users", nil); !strings.Contains(body, "bob@example.com") {
		t.Errorf("list users = %s", body)
	}

	bc := e.client(t)
	e.do(t, bc, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "bob@example.com", "password": "bobs long password"})
	if code, _, _ := e.do(t, bc, http.MethodGet, "/api/v1/users", nil); code != http.StatusForbidden {
		t.Errorf("member listing users: %d, want 403", code)
	}

	var me struct {
		Data auth.User `json:"data"`
	}
	_, body, _ = e.do(t, ac, http.MethodGet, "/api/v1/auth/me", nil)
	_ = json.Unmarshal([]byte(body), &me)
	if code, _, _ := e.do(t, ac, http.MethodDelete, "/api/v1/users/"+me.Data.ID, nil); code != http.StatusBadRequest {
		t.Errorf("deleting yourself: %d, want 400", code)
	}
	if code, _, _ := e.do(t, ac, http.MethodDelete, "/api/v1/users/"+bob.Data.ID, nil); code != http.StatusNoContent {
		t.Errorf("delete bob: %d", code)
	}
	if code, _, _ := e.do(t, bc, http.MethodGet, "/api/v1/data", nil); code != http.StatusUnauthorized {
		t.Errorf("deleted user still signed in: %d", code)
	}
}

func TestChangeOwnPassword(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	e.do(t, c, http.MethodPost, "/api/v1/auth/setup", admin)

	code, _, _ := e.do(t, c, http.MethodPut, "/api/v1/auth/me/password",
		map[string]string{"current_password": "wrong wrong wrong", "new_password": "an even better password"})
	if code != http.StatusUnauthorized {
		t.Errorf("wrong current password: %d, want 401", code)
	}
	code, _, _ = e.do(t, c, http.MethodPut, "/api/v1/auth/me/password",
		map[string]string{"current_password": "correct horse battery", "new_password": "an even better password"})
	if code != http.StatusNoContent {
		t.Fatalf("change password: %d", code)
	}
	// The browser that changed it stays signed in with a fresh session.
	if code, _, _ := e.do(t, c, http.MethodGet, "/api/v1/data", nil); code != http.StatusOK {
		t.Errorf("after password change: %d", code)
	}
}
