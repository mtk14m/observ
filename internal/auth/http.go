package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CookieName is the session cookie.
const CookieName = "obsrv_session"

// HandlerOptions configures the HTTP layer.
type HandlerOptions struct {
	// SecureCookies marks the session cookie Secure (HTTPS only).
	SecureCookies bool
	// SessionTTL is the cookie lifetime; it should match the store's.
	SessionTTL time.Duration
	Now        func() time.Time
}

// Handler serves /api/v1/auth and /api/v1/users, and protects other routes.
type Handler struct {
	store   *Store
	opts    HandlerOptions
	limiter *limiter
}

// NewHandler returns the authentication HTTP handler.
func NewHandler(store *Store, opts HandlerOptions) *Handler {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 30 * 24 * time.Hour
	}
	return &Handler{store: store, opts: opts, limiter: newLimiter(5, 15*time.Minute, opts.Now)}
}

type ctxKey struct{}

// UserFrom returns the signed-in user of a request served behind Require.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Register adds the authentication and user management routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/status", h.status)
	mux.Handle("POST /api/v1/auth/setup", sameOrigin(http.HandlerFunc(h.setup)))
	mux.Handle("POST /api/v1/auth/login", sameOrigin(http.HandlerFunc(h.login)))
	mux.Handle("POST /api/v1/auth/logout", sameOrigin(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/v1/auth/me", h.Require(http.HandlerFunc(h.me)))
	mux.Handle("PUT /api/v1/auth/me/password", h.Require(http.HandlerFunc(h.changePassword)))
	mux.Handle("GET /api/v1/users", h.Require(adminOnly(http.HandlerFunc(h.listUsers))))
	mux.Handle("POST /api/v1/users", h.Require(adminOnly(http.HandlerFunc(h.createUser))))
	mux.Handle("DELETE /api/v1/users/{id}", h.Require(adminOnly(http.HandlerFunc(h.deleteUser))))
}

// Require serves next only to signed-in users, and rejects cross-origin
// writes (CSRF protection on top of SameSite cookies).
func (h *Handler) Require(next http.Handler) http.Handler {
	return sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "sign in required")
			return
		}
		u, err := h.store.SessionUser(r.Context(), c.Value)
		if err != nil {
			if !errors.Is(err, ErrNoSession) {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			h.clearCookie(w)
			writeErr(w, http.StatusUnauthorized, "sign in required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	}))
}

func adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, _ := UserFrom(r.Context()); u.Role != RoleAdmin {
			writeErr(w, http.StatusForbidden, "only admins can manage users")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin rejects state-changing requests sent by another site. Requests
// without an Origin header (curl, scripts) are allowed: they cannot carry a
// victim's cookies.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					writeErr(w, http.StatusForbidden, "cross-origin request rejected")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"setup_required": n == 0})
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	var in NewUser
	if !decode(w, r, &in) {
		return
	}
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n > 0 {
		writeErr(w, http.StatusConflict, "obsrv is already set up: sign in instead")
		return
	}
	in.Role = RoleAdmin
	u, err := h.store.CreateUser(r.Context(), in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusCreated, u)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	key := normalizeEmail(in.Email) + "|" + clientIP(r)
	if wait := h.limiter.blocked(key); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeErr(w, http.StatusTooManyRequests, "too many failed attempts, try again later")
		return
	}
	u, err := h.store.Authenticate(r.Context(), in.Email, in.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		h.limiter.fail(key)
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.limiter.succeed(key)
	if err := h.startSession(w, r, u); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, u)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = h.store.DeleteSession(r.Context(), c.Value)
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	writeData(w, http.StatusOK, u)
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, _ := UserFrom(r.Context())
	if _, err := h.store.Authenticate(r.Context(), u.Email, in.Current); err != nil {
		writeErr(w, http.StatusUnauthorized, "the current password is wrong")
		return
	}
	if err := h.store.SetPassword(r.Context(), u.ID, in.New); err != nil {
		writeStoreErr(w, err)
		return
	}
	// Changing the password ends every session; keep this browser signed in.
	if err := h.startSession(w, r, u); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, users)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in NewUser
	if !decode(w, r, &in) {
		return
	}
	u, err := h.store.CreateUser(r.Context(), in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeData(w, http.StatusCreated, u)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	me, _ := UserFrom(r.Context())
	id := r.PathValue("id")
	if id == me.ID {
		writeErr(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, u User) error {
	token, err := h.store.CreateSession(r.Context(), u.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: h.opts.SecureCookies, MaxAge: int(h.opts.SessionTTL.Seconds()),
	})
	return nil
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: h.opts.SecureCookies, MaxAge: -1,
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err == nil {
		err = json.Unmarshal(body, v)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return false
	}
	return true
}

func writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "auth: invalid: "))
	case errors.Is(err, ErrExists):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func writeData(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
