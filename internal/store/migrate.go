package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrate applies every embedded migration that the database has not seen,
// tracked with PRAGMA user_version. Each file runs inside a transaction that
// also bumps the version, so a failed migration leaves nothing half-applied.
func (s *Store) migrate(ctx context.Context) error {
	names, err := migrationNames()
	if err != nil {
		return err
	}

	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > len(names) {
		return fmt.Errorf("database schema version %d is newer than this binary knows about (%d)", version, len(names))
	}

	for i := version; i < len(names); i++ {
		body, err := migrationFS.ReadFile(names[i])
		if err != nil {
			return fmt.Errorf("read migration %s: %w", names[i], err)
		}
		next := i + 1
		err = s.tx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("apply migration %s: %w", names[i], err)
			}
			// PRAGMA does not accept a bound parameter; next is an int we control.
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, next)); err != nil {
				return fmt.Errorf("record schema version %d: %w", next, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func migrationNames() ([]string, error) {
	entries, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	return entries, nil
}
