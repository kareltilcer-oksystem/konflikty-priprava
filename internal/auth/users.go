// Package auth holds the three fixed accounts and the session token helper.
// There is no user table: records reference the username string (PRD 4).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// Role separates "can edit" from "can prepare the meeting".
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
)

// Account is one entry from AUTH_USERS.
type Account struct {
	Username    string
	DisplayName string
	Role        Role

	pwHash [sha256.Size]byte
}

// IsAdmin reports whether the account may prepare meetings.
func (a Account) IsAdmin() bool { return a.Role == RoleAdmin }

// Registry is the parsed, immutable set of accounts.
type Registry struct {
	byName map[string]Account
	order  []string

	// dummy is compared against when the username is unknown, so a present and
	// an absent account take the same code path and cost the same time.
	dummy [sha256.Size]byte
}

// ParseUsers reads the AUTH_USERS variable: accounts separated by ';', each
// spelled username:password:Display Name:role.
//
// An account that does not split into exactly four fields is rejected rather
// than guessed at. A stray ':' — most likely in a display name written as
// "Petr Admin, CTO:" — would otherwise shift every later field by one and land
// "CTO" in the role; a stray ';' would split one account into two malformed ones.
//
// Error messages name the offending account by its first field only and never
// echo the segment, because AUTH_USERS contains plaintext passwords and these
// errors are printed to the log at start-up.
func ParseUsers(raw string) (*Registry, error) {
	reg := &Registry{byName: make(map[string]Account)}
	if _, err := rand.Read(reg.dummy[:]); err != nil {
		return nil, fmt.Errorf("generate dummy digest: %w", err)
	}

	var admins int
	var segIndex int
	for _, seg := range strings.Split(raw, ";") {
		if strings.TrimSpace(seg) == "" {
			// Tolerate a trailing ';' — a very easy thing to leave in a .env file.
			continue
		}
		segIndex++
		fields := strings.Split(seg, ":")
		if len(fields) != 4 {
			return nil, fmt.Errorf("AUTH_USERS account #%d (%s) has %d fields, expected exactly 4 "+
				"(username:password:Display Name:role); no field may contain ':' or ';'",
				segIndex, describe(fields), len(fields))
		}
		username := strings.TrimSpace(fields[0])
		password := fields[1]
		display := strings.TrimSpace(fields[2])
		role := Role(strings.TrimSpace(fields[3]))

		switch {
		case username == "":
			return nil, fmt.Errorf("AUTH_USERS account #%d has an empty username", segIndex)
		case password == "":
			return nil, fmt.Errorf("AUTH_USERS account %q has an empty password", username)
		case display == "":
			return nil, fmt.Errorf("AUTH_USERS account %q has an empty display name", username)
		case role != RoleAdmin && role != RoleEditor:
			return nil, fmt.Errorf("AUTH_USERS account %q has role %q, expected 'admin' or 'editor'", username, role)
		}
		if _, dup := reg.byName[username]; dup {
			return nil, fmt.Errorf("AUTH_USERS lists the username %q twice", username)
		}
		if role == RoleAdmin {
			admins++
		}
		reg.byName[username] = Account{
			Username:    username,
			DisplayName: display,
			Role:        role,
			pwHash:      sha256.Sum256([]byte(password)),
		}
		reg.order = append(reg.order, username)
	}

	if len(reg.order) == 0 {
		return nil, errors.New("AUTH_USERS defines no accounts")
	}
	if admins == 0 {
		// Without an admin, meetings could never be created: half the app would
		// be unreachable.
		return nil, errors.New("AUTH_USERS defines no account with role 'admin'")
	}
	return reg, nil
}

// describe names an account for an error message without revealing any secret.
// Only the first field — the username — is ever quoted.
func describe(fields []string) string {
	if len(fields) == 0 || strings.TrimSpace(fields[0]) == "" {
		return "username unknown"
	}
	return fmt.Sprintf("username %q", strings.TrimSpace(fields[0]))
}

// Count returns the number of configured accounts.
func (r *Registry) Count() int { return len(r.order) }

// AdminCount returns how many accounts carry the admin role. Exactly one is
// expected; more is unusual but not an error.
func (r *Registry) AdminCount() int {
	var n int
	for _, name := range r.order {
		if r.byName[name].IsAdmin() {
			n++
		}
	}
	return n
}

// Lookup returns the account for a username. It is how a session token is
// resolved back to a user, so an account removed from AUTH_USERS stops being
// able to act the moment the binary restarts.
func (r *Registry) Lookup(username string) (Account, bool) {
	a, ok := r.byName[username]
	return a, ok
}

// Verify checks a username/password pair in constant time.
//
// The comparison is over SHA-256 digests so it does not leak the password's
// length, and an unknown username is compared against a per-process random
// digest so it takes the same path as a wrong password. The passwords are still
// plaintext in the environment by design (PRD non-goal N1, NFR2).
func (r *Registry) Verify(username, password string) (Account, bool) {
	given := sha256.Sum256([]byte(password))
	acct, found := r.byName[username]
	want := r.dummy
	if found {
		want = acct.pwHash
	}
	if subtle.ConstantTimeCompare(given[:], want[:]) != 1 || !found {
		return Account{}, false
	}
	return acct, true
}
