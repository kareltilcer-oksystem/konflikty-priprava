package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// login verifies credentials and starts a session.
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, msgBadJSON)
		return
	}

	account, ok := a.users.Verify(req.Username, req.Password)
	if !ok {
		// A failed attempt is answered slowly so scripted guessing is boring.
		// There is no lockout (FR-A6). The wait is abandoned if the client goes
		// away, so it cannot be used to pin down connections.
		a.delayFailedLogin(r)
		writeError(w, http.StatusUnauthorized, CodeInvalidCredentials, msgInvalidCredentials)
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		slog.Error("create session token", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}
	now := a.now()
	if err := a.store.CreateSession(r.Context(), token, account.Username, now, now.Add(a.cfg.SessionTTL)); err != nil {
		slog.Error("create session", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, msgInternal)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, toUser(account))
}

func (a *API) delayFailedLogin(r *http.Request) {
	if a.loginDelay <= 0 {
		return
	}
	t := time.NewTimer(a.loginDelay)
	defer t.Stop()
	select {
	case <-t.C:
	case <-r.Context().Done():
	}
}

// logout invalidates the session. Calling it without one is a no-op.
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if token := tokenFrom(r.Context()); token != "" {
		if err := a.store.DeleteSession(r.Context(), token); err != nil {
			slog.Error("delete session", "err", err)
		}
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// me returns the signed-in user, or JSON null for an anonymous visitor. The
// frontend calls it on start-up to decide which controls to render.
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, toUser(account))
}

// listUsers returns the configured accounts so the UI can render an author's
// display name instead of the bare username it is stored under.
//
// Public, like every other read: author names are already visible to anyone
// (PRD decision log, Q12). It carries no secret — only what AUTH_USERS made
// public — and it is a name lookup, not the user management the PRD rules out.
func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	accounts := a.users.All()
	out := make([]userDTO, 0, len(accounts))
	for _, acct := range accounts {
		out = append(out, toUser(acct))
	}
	writeJSON(w, http.StatusOK, out)
}
