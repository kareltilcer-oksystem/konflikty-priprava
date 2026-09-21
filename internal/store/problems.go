package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// problemColumns selects a problem together with its two derived counts. Both
// counts are correlated subqueries rather than joins, which keeps the row count
// honest when a problem has several attachments and several meetings at once.
const problemColumns = `
  p.id, p.title, p.description, p.link, p.created_at, p.created_by, p.updated_at,
  p.done, p.done_at, p.done_by,
  (SELECT COUNT(*) FROM attachments   a WHERE a.problem_id = p.id) AS attachment_count,
  (SELECT COUNT(*) FROM meeting_items m WHERE m.problem_id = p.id) AS meeting_count`

type scanner interface {
	Scan(dest ...any) error
}

func scanProblem(sc scanner) (Problem, error) {
	var p Problem
	var doneAt, doneBy sql.NullString
	err := sc.Scan(&p.ID, &p.Title, &p.Description, &p.Link, &p.CreatedAt, &p.CreatedBy,
		&p.UpdatedAt, &p.Done, &doneAt, &doneBy, &p.AttachmentCount, &p.MeetingCount)
	if err != nil {
		return Problem{}, err
	}
	p.DoneAt, p.DoneBy = nullString(doneAt), nullString(doneBy)
	return p, nil
}

// ListProblems returns the bucket. The text filter q is applied by the caller
// after this returns — SQLite cannot fold Czech diacritics.
func (s *Store) ListProblems(ctx context.Context, f ProblemFilter) ([]Problem, error) {
	where := ""
	var args []any
	add := func(clause string, a ...any) {
		if where == "" {
			where = " WHERE " + clause
		} else {
			where += " AND " + clause
		}
		args = append(args, a...)
	}
	if f.Done != nil {
		add("p.done = ?", boolInt(*f.Done))
	}
	if f.Scheduled != nil {
		if *f.Scheduled {
			add("EXISTS (SELECT 1 FROM meeting_items m WHERE m.problem_id = p.id)")
		} else {
			add("NOT EXISTS (SELECT 1 FROM meeting_items m WHERE m.problem_id = p.id)")
		}
	}

	// The contract names no tiebreaker, which would leave the order of equal
	// rows unstable between identical requests. id settles it.
	order := "ORDER BY p.created_at DESC, p.id DESC"
	switch f.Sort {
	case SortCreatedAsc:
		order = "ORDER BY p.created_at ASC, p.id ASC"
	case SortMeetingsDesc:
		order = "ORDER BY meeting_count DESC, p.created_at DESC, p.id DESC"
	}

	query := "SELECT" + problemColumns + " FROM problems p" + where + " " + order
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list problems: %w", err)
	}
	defer rows.Close()

	out := []Problem{}
	for rows.Next() {
		p, err := scanProblem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan problem: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProblem returns one problem, or ErrNotFound.
func (s *Store) GetProblem(ctx context.Context, id int64) (Problem, error) {
	query := "SELECT" + problemColumns + " FROM problems p WHERE p.id = ?"
	p, err := scanProblem(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Problem{}, ErrNotFound
	}
	if err != nil {
		return Problem{}, fmt.Errorf("get problem %d: %w", id, err)
	}
	return p, nil
}

// CreateProblem inserts a problem and its already-staged attachments in one
// transaction, so a rejected upload never leaves a half-made problem behind.
func (s *Store) CreateProblem(ctx context.Context, in ProblemInput, atts []NewAttachment, by string, now time.Time) (int64, error) {
	ts := rfc3339(now)
	var id int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO problems (title, description, link, created_at, created_by, updated_at, done)
			 VALUES (?, ?, ?, ?, ?, ?, 0)`,
			in.Title, in.Description, in.Link, ts, by, ts)
		if err != nil {
			return fmt.Errorf("insert problem: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("read new problem id: %w", err)
		}
		return insertAttachments(ctx, tx, id, atts, by, ts)
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateProblem applies a partial update and appends any newly staged
// attachments, in one transaction. Existing attachments are untouched.
func (s *Store) UpdateProblem(ctx context.Context, id int64, patch ProblemPatch, atts []NewAttachment, by string, now time.Time) error {
	ts := rfc3339(now)
	return s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM problems WHERE id = ?`, id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up problem %d: %w", id, err)
		}

		set := "updated_at = ?"
		args := []any{ts}
		if patch.Title != nil {
			set += ", title = ?"
			args = append(args, *patch.Title)
		}
		if patch.Description != nil {
			set += ", description = ?"
			args = append(args, *patch.Description)
		}
		if patch.Link != nil {
			set += ", link = ?"
			args = append(args, *patch.Link)
		}
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, `UPDATE problems SET `+set+` WHERE id = ?`, args...); err != nil {
			return fmt.Errorf("update problem %d: %w", id, err)
		}
		return insertAttachments(ctx, tx, id, atts, by, ts)
	})
}

// SetProblemDone flips the done flag. It is idempotent by construction: the
// guard on the current value means marking an already-done problem done again
// does not rewrite done_at / done_by, so it cannot rewrite history.
func (s *Store) SetProblemDone(ctx context.Context, id int64, done bool, by string, now time.Time) error {
	ts := rfc3339(now)
	return s.tx(ctx, func(tx *sql.Tx) error {
		var current bool
		if err := tx.QueryRowContext(ctx, `SELECT done FROM problems WHERE id = ?`, id).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up problem %d: %w", id, err)
		}
		if current == done {
			return nil
		}
		var err error
		if done {
			_, err = tx.ExecContext(ctx,
				`UPDATE problems SET done = 1, done_at = ?, done_by = ?, updated_at = ? WHERE id = ? AND done = 0`,
				ts, by, ts, id)
		} else {
			_, err = tx.ExecContext(ctx,
				`UPDATE problems SET done = 0, done_at = NULL, done_by = NULL, updated_at = ? WHERE id = ? AND done = 1`,
				ts, id)
		}
		if err != nil {
			return fmt.Errorf("set done on problem %d: %w", id, err)
		}
		return nil
	})
}

// DeleteProblem removes a problem and everything referencing it, and returns
// the storage names of the attachment files the caller must unlink.
//
// The cascade takes the attachment rows and the agenda entries, but SQLite
// cannot remove bytes from disk and does not renumber the agendas it has just
// punched holes in. Both are handled here: every meeting that lost an entry is
// renumbered inside the same transaction, and the storage names are handed back
// so the handler can unlink the files once the commit has succeeded.
func (s *Store) DeleteProblem(ctx context.Context, id int64) ([]string, error) {
	var storageNames []string
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM problems WHERE id = ?`, id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up problem %d: %w", id, err)
		}

		meetingIDs, err := scanInt64s(ctx, tx,
			`SELECT DISTINCT meeting_id FROM meeting_items WHERE problem_id = ?`, id)
		if err != nil {
			return fmt.Errorf("collect affected meetings: %w", err)
		}
		storageNames, err = scanStrings(ctx, tx,
			`SELECT storage_name FROM attachments WHERE problem_id = ?`, id)
		if err != nil {
			return fmt.Errorf("collect attachment files: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM problems WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete problem %d: %w", id, err)
		}
		for _, mid := range meetingIDs {
			if err := renumberItems(ctx, tx, mid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return storageNames, nil
}

func insertAttachments(ctx context.Context, tx *sql.Tx, problemID int64, atts []NewAttachment, by, ts string) error {
	for _, a := range atts {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO attachments (problem_id, filename, content_type, size_bytes, storage_name, created_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			problemID, a.Filename, a.ContentType, a.SizeBytes, a.StorageName, ts, by)
		if err != nil {
			return fmt.Errorf("insert attachment %q: %w", a.Filename, err)
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanInt64s(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanStrings(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ProblemsByIDs loads several problems at once, keyed by id.
//
// The agenda page needs one problem per item; this keeps that at a single
// query however long the agenda is.
func (s *Store) ProblemsByIDs(ctx context.Context, ids []int64) (map[int64]Problem, error) {
	out := make(map[int64]Problem, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := "SELECT" + problemColumns + " FROM problems p WHERE p.id IN (" + placeholders(len(ids)) + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load problems by id: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanProblem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan problem: %w", err)
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}
