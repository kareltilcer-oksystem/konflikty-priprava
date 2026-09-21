package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/slug"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/timeutil"
)

const meetingColumns = `
  m.id, m.slug, m.meeting_date, m.note, m.created_at, m.created_by,
  (SELECT COUNT(*) FROM meeting_items i WHERE i.meeting_id = m.id) AS item_count`

func scanMeeting(sc scanner) (Meeting, error) {
	var m Meeting
	err := sc.Scan(&m.ID, &m.Slug, &m.MeetingDate, &m.Note, &m.CreatedAt, &m.CreatedBy, &m.ItemCount)
	return m, err
}

// ListMeetings returns meetings newest first. sinceDate, when non-empty, keeps
// only meetings on or after it.
//
// The filter is deliberately one-sided. A meeting dated in the future must
// never be filtered out: the one being prepared for the coming week is always
// future-dated and is the row the list exists to surface.
func (s *Store) ListMeetings(ctx context.Context, sinceDate string) ([]Meeting, error) {
	query := "SELECT" + meetingColumns + " FROM meetings m"
	var args []any
	if sinceDate != "" {
		query += " WHERE m.meeting_date >= ?"
		args = append(args, sinceDate)
	}
	query += " ORDER BY m.meeting_date DESC, m.id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	defer rows.Close()
	out := []Meeting{}
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan meeting: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMeetingBySlug returns one meeting, or ErrNotFound. Archived meetings are
// returned like any other: a link shared three years ago still opens.
func (s *Store) GetMeetingBySlug(ctx context.Context, meetingSlug string) (Meeting, error) {
	query := "SELECT" + meetingColumns + " FROM meetings m WHERE m.slug = ?"
	m, err := scanMeeting(s.db.QueryRowContext(ctx, query, meetingSlug))
	if errors.Is(err, sql.ErrNoRows) {
		return Meeting{}, ErrNotFound
	}
	if err != nil {
		return Meeting{}, fmt.Errorf("get meeting %q: %w", meetingSlug, err)
	}
	return m, nil
}

// CreateMeeting inserts a meeting, deriving and freezing its slug.
//
// The slug is allocated inside the transaction: the smallest free suffix for
// the date's ISO week is chosen, and a UNIQUE violation retries the whole
// block. The retry only matters if the single-connection limit is ever relaxed,
// but it costs nothing and makes the allocation correct on its own terms.
func (s *Store) CreateMeeting(ctx context.Context, date time.Time, note, by string, now time.Time) (Meeting, error) {
	ts := rfc3339(now)
	dateStr := timeutil.FormatDate(date)
	base := slug.Base(date)

	var created Meeting
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := s.tx(ctx, func(tx *sql.Tx) error {
			taken, err := scanStrings(ctx, tx,
				`SELECT slug FROM meetings WHERE slug = ? OR slug LIKE ?`, base, base+"-%")
			if err != nil {
				return fmt.Errorf("look up existing slugs: %w", err)
			}
			used := make(map[string]bool, len(taken))
			for _, t := range taken {
				used[t] = true
			}
			n := 1
			for used[slug.WithSuffix(base, n)] {
				n++
			}
			newSlug := slug.WithSuffix(base, n)

			res, err := tx.ExecContext(ctx,
				`INSERT INTO meetings (slug, meeting_date, note, created_at, created_by) VALUES (?, ?, ?, ?, ?)`,
				newSlug, dateStr, note, ts, by)
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("read new meeting id: %w", err)
			}
			created = Meeting{
				ID: id, Slug: newSlug, MeetingDate: dateStr, Note: note,
				ItemCount: 0, CreatedAt: ts, CreatedBy: by,
			}
			return nil
		})
		if err == nil {
			return created, nil
		}
		if isUniqueViolation(err) {
			continue // another writer took the suffix; pick the next one
		}
		return Meeting{}, fmt.Errorf("create meeting: %w", err)
	}
	return Meeting{}, fmt.Errorf("create meeting: could not allocate a slug for %s after %d attempts", base, maxAttempts)
}

// UpdateMeeting applies a partial update.
//
// The slug is never regenerated when the date changes, so links shared earlier
// keep working. iso_year and iso_week are always derived from the current
// meeting_date on read, so after a date change the week label follows the new
// date while the URL keeps the old slug. That divergence is deliberate.
func (s *Store) UpdateMeeting(ctx context.Context, meetingSlug string, patch MeetingPatch) (Meeting, error) {
	var out Meeting
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM meetings WHERE slug = ?`, meetingSlug).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up meeting %q: %w", meetingSlug, err)
		}
		var sets []string
		var args []any
		if patch.MeetingDate != nil {
			sets = append(sets, "meeting_date = ?")
			args = append(args, *patch.MeetingDate)
		}
		if patch.Note != nil {
			sets = append(sets, "note = ?")
			args = append(args, *patch.Note)
		}
		if len(sets) > 0 {
			query := "UPDATE meetings SET " + joinComma(sets) + " WHERE id = ?"
			args = append(args, id)
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return fmt.Errorf("update meeting %q: %w", meetingSlug, err)
			}
		}
		var err error
		out, err = scanMeeting(tx.QueryRowContext(ctx, "SELECT"+meetingColumns+" FROM meetings m WHERE m.id = ?", id))
		if err != nil {
			return fmt.Errorf("reload meeting %q: %w", meetingSlug, err)
		}
		return nil
	})
	if err != nil {
		return Meeting{}, err
	}
	return out, nil
}

// DeleteMeeting removes a meeting, its agenda entries and their notes. The
// problems themselves are untouched and stay in the bucket.
func (s *Store) DeleteMeeting(ctx context.Context, meetingSlug string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM meetings WHERE slug = ?`, meetingSlug)
		if err != nil {
			return fmt.Errorf("delete meeting %q: %w", meetingSlug, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("delete meeting %q: %w", meetingSlug, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
