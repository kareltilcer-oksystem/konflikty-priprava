package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const attachmentColumns = `id, problem_id, filename, content_type, size_bytes, storage_name, created_at, created_by`

func scanAttachment(sc scanner) (Attachment, error) {
	var a Attachment
	err := sc.Scan(&a.ID, &a.ProblemID, &a.Filename, &a.ContentType, &a.SizeBytes,
		&a.StorageName, &a.CreatedAt, &a.CreatedBy)
	return a, err
}

// AttachmentsByProblem returns one problem's files, oldest first.
func (s *Store) AttachmentsByProblem(ctx context.Context, problemID int64) ([]Attachment, error) {
	byID, err := s.AttachmentsByProblems(ctx, []int64{problemID})
	if err != nil {
		return nil, err
	}
	if out := byID[problemID]; out != nil {
		return out, nil
	}
	return []Attachment{}, nil
}

// AttachmentsByProblems loads the files of several problems in one query.
//
// The agenda page embeds every item's attachments, so fetching them per item
// would issue one query per agenda entry. This keeps GET /meetings/{slug} at a
// fixed number of round trips however long the agenda is.
func (s *Store) AttachmentsByProblems(ctx context.Context, problemIDs []int64) (map[int64][]Attachment, error) {
	out := make(map[int64][]Attachment, len(problemIDs))
	if len(problemIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(problemIDs))
	for i, id := range problemIDs {
		args[i] = id
	}
	query := `SELECT ` + attachmentColumns + ` FROM attachments
	          WHERE problem_id IN (` + placeholders(len(problemIDs)) + `)
	          ORDER BY problem_id, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out[a.ProblemID] = append(out[a.ProblemID], a)
	}
	return out, rows.Err()
}

// GetAttachment returns one attachment's metadata, or ErrNotFound.
func (s *Store) GetAttachment(ctx context.Context, id int64) (Attachment, error) {
	query := `SELECT ` + attachmentColumns + ` FROM attachments WHERE id = ?`
	a, err := scanAttachment(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("get attachment %d: %w", id, err)
	}
	return a, nil
}

// AddAttachment inserts one already-staged file against an existing problem and
// bumps the problem's updated_at.
func (s *Store) AddAttachment(ctx context.Context, problemID int64, a NewAttachment, by string, now time.Time) (Attachment, error) {
	ts := rfc3339(now)
	var id int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM problems WHERE id = ?`, problemID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up problem %d: %w", problemID, err)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO attachments (problem_id, filename, content_type, size_bytes, storage_name, created_at, created_by)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			problemID, a.Filename, a.ContentType, a.SizeBytes, a.StorageName, ts, by)
		if err != nil {
			return fmt.Errorf("insert attachment: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("read new attachment id: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE problems SET updated_at = ? WHERE id = ?`, ts, problemID); err != nil {
			return fmt.Errorf("touch problem %d: %w", problemID, err)
		}
		return nil
	})
	if err != nil {
		return Attachment{}, err
	}
	return Attachment{
		ID: id, ProblemID: problemID, Filename: a.Filename, ContentType: a.ContentType,
		SizeBytes: a.SizeBytes, StorageName: a.StorageName, CreatedAt: ts, CreatedBy: by,
	}, nil
}

// DeleteAttachment removes the metadata row and returns the storage name of the
// file the caller must unlink.
func (s *Store) DeleteAttachment(ctx context.Context, id int64) (string, error) {
	var storageName string
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var problemID int64
		err := tx.QueryRowContext(ctx,
			`SELECT storage_name, problem_id FROM attachments WHERE id = ?`, id).
			Scan(&storageName, &problemID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("look up attachment %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete attachment %d: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return storageName, nil
}
