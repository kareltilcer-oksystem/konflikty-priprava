package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateSession stores a new session row.
func (s *Store) CreateSession(ctx context.Context, token, username string, now, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, username, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token, username, rfc3339(now), rfc3339(expires))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SlideSession validates a token and extends its expiry in one round trip,
// returning the username it belongs to.
//
// Expiry check and extension happen in a single statement so there is no window
// between the two. The caller re-sends the cookie with a matching Max-Age on
// every request: setting the cookie's lifetime only at login would log an
// actively used session out on day 30 however far the server-side row had been
// extended.
//
// An absent or expired token is ErrNotFound — the request is simply anonymous.
func (s *Store) SlideSession(ctx context.Context, token string, now, expires time.Time) (string, error) {
	var username string
	err := s.db.QueryRowContext(ctx,
		`UPDATE sessions SET expires_at = ? WHERE token = ? AND expires_at > ? RETURNING username`,
		rfc3339(expires), token, rfc3339(now)).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("refresh session: %w", err)
	}
	return username, nil
}

// DeleteSession invalidates a token. Deleting one that does not exist is not an
// error: logging out without a session is a documented no-op.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PurgeExpiredSessions removes rows that have already lapsed. Called at
// start-up and once a day.
func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, rfc3339(now))
	if err != nil {
		return 0, fmt.Errorf("purge expired sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge expired sessions: %w", err)
	}
	return n, nil
}
