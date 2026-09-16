// Package store is the SQLite persistence layer. Every exported method is safe
// to call concurrently; the pool is deliberately limited to a single connection.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, builds cleanly on Windows
)

// Store owns the database handle.
type Store struct {
	db *sql.DB
}

// Open connects to the database at path, applies the pragmas and runs the
// migrations.
//
// The pool is capped at one connection. With three users and sub-millisecond
// queries, serialising the whole database costs nothing measurable and removes
// every SQLITE_BUSY and lock-upgrade failure mode at a stroke. Attachment bytes
// never pass through a DB connection, so the single connection is never held
// during a 100 MB upload or a video stream.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// dsn builds the connection string. The pragmas are part of the DSN so they
// apply to every connection the pool ever opens, not just the first.
//
// _txlock=immediate takes the write lock when a transaction begins rather than
// on its first write, which keeps the slug allocation correct even if the
// single-connection limit is ever relaxed.
func dsn(path string) string {
	p := filepath.ToSlash(path)
	return "file:" + p +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_txlock=immediate"
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for tests that need to assert on raw state.
func (s *Store) DB() *sql.DB { return s.db }

// tx runs fn inside a transaction, rolling back on error or panic.
func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	t, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = t.Rollback()
			panic(p)
		}
	}()
	if err := fn(t); err != nil {
		_ = t.Rollback()
		return err
	}
	if err := t.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// rfc3339 renders an instant the way every timestamp column stores it.
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// placeholders builds "?, ?, ?" for an IN clause of n values.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// nullString converts a nullable column into a *string for the wire.
func nullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
