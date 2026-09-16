// Package httpapi implements the REST API under /api.
//
// One handler serves both listeners: the API port and, in production, the /api
// prefix of the SPA port. Route patterns therefore carry the literal /api
// prefix and no prefix stripping exists anywhere.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/auth"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/config"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

// SessionCookie is the name of the opaque session cookie.
const SessionCookie = "session"

// Deps is everything the API needs from the rest of the program.
type Deps struct {
	Store   *store.Store
	Users   *auth.Registry
	Config  *config.Config
	Version string

	// Now is injectable so tests can pin dates; production passes time.Now.
	Now func() time.Time

	// LoginDelay slows a failed login so scripted guessing is boring (FR-A6).
	// Tests set it to zero.
	LoginDelay time.Duration
}

// API holds the resolved dependencies behind the handlers.
type API struct {
	store   *store.Store
	users   *auth.Registry
	cfg     *config.Config
	version string
	now     func() time.Time

	loginDelay time.Duration
}

// New builds the /api handler. The returned handler is safe to mount on more
// than one listener.
func New(d Deps) http.Handler {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	a := &API{
		store:      d.Store,
		users:      d.Users,
		cfg:        d.Config,
		version:    d.Version,
		now:        now,
		loginDelay: d.LoginDelay,
	}
	return a.routes()
}

// contextKey is unexported so no other package can collide with these keys.
type contextKey int

const (
	ctxAccount contextKey = iota
	ctxToken
)

// accountFrom returns the signed-in account, if any.
func accountFrom(ctx context.Context) (auth.Account, bool) {
	a, ok := ctx.Value(ctxAccount).(auth.Account)
	return a, ok
}

// tokenFrom returns the session token carried by the request, if any.
func tokenFrom(ctx context.Context) string {
	t, _ := ctx.Value(ctxToken).(string)
	return t
}
