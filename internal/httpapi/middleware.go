package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

// NOTE: no middleware here may call r.FormValue or r.ParseForm. Either one
// consumes the body, after which r.MultipartReader() fails and every upload
// breaks.

// withSession resolves the session cookie, slides its expiry and re-issues the
// cookie.
//
// It wraps everything under /api, not just the mutating routes: GET /auth/me
// needs it, and the session slides on every authenticated request including
// reads (FR-A5).
func (a *API) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		now := a.now()
		expires := now.Add(a.cfg.SessionTTL)

		username, err := a.store.SlideSession(r.Context(), cookie.Value, now, expires)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Error("refresh session", "err", err)
			}
			// Absent or expired: the request is simply anonymous.
			a.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}
		account, ok := a.users.Lookup(username)
		if !ok {
			// The account was removed from AUTH_USERS since the session began.
			_ = a.store.DeleteSession(r.Context(), cookie.Value)
			a.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}

		// Re-send the cookie with a fresh Max-Age. Setting its lifetime only at
		// login would log an actively used session out on day 30 however far the
		// server-side row had been extended.
		a.setSessionCookie(w, cookie.Value)

		ctx := context.WithValue(r.Context(), ctxAccount, account)
		ctx = context.WithValue(ctx, ctxToken, cookie.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is deliberately false: the app runs over plain HTTP on an
		// internal network (PRD NFR1), and a Secure cookie would never be sent.
		MaxAge: int(a.cfg.SessionTTL / time.Second),
	})
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// requireAuth rejects anonymous requests.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := accountFrom(r.Context()); !ok {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, msgUnauthorized)
			return
		}
		next(w, r)
	}
}

// requireAdmin rejects anyone who is not the admin.
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, msgUnauthorized)
			return
		}
		if !account.IsAdmin() {
			writeError(w, http.StatusForbidden, CodeForbidden, msgForbidden)
			return
		}
		next(w, r)
	}
}

// Recover turns a panic into a 500 instead of a dropped connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("panic serving request", "method", r.Method, "path", r.URL.Path, "panic", p)
				writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Log records one line per request.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}

// statusRecorder remembers the status code for the log line.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

// ReadFrom keeps the underlying writer's fast path available.
//
// Without it the wrapper hides io.ReaderFrom, and every attachment download —
// including a 100 MB video served through http.ServeContent — falls back to
// userspace copying.
func (s *statusRecorder) ReadFrom(src io.Reader) (int64, error) {
	if rf, ok := s.ResponseWriter.(io.ReaderFrom); ok {
		if !s.wroteHeader {
			s.wroteHeader = true
		}
		return rf.ReadFrom(src)
	}
	return io.Copy(s.ResponseWriter, src)
}

// Flush preserves streaming for any handler that needs it.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
