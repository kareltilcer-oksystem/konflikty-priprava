package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const itemColumns = `id, meeting_id, problem_id, position, prep_note, action_note, created_at`

func scanItem(sc scanner) (MeetingItem, error) {
	var it MeetingItem
	err := sc.Scan(&it.ID, &it.MeetingID, &it.ProblemID, &it.Position, &it.PrepNote, &it.ActionNote, &it.CreatedAt)
	return it, err
}

// renumberItems rewrites one meeting's positions so they are contiguous from 0,
// preserving the existing order.
//
// This runs after any delete that punches a hole in an agenda. SQLite's cascade
// removes the row but leaves the survivors numbered 0, 2, 3 — which the UI
// would render as 1, 3, 4.
//
// The order is read first and the new positions are written explicitly, rather
// than computed by a correlated subquery inside one UPDATE. That shortcut is
// wrong: SQLite walks the table in rowid order and the subquery sees rows the
// same statement has already rewritten, so once the agenda has been reordered —
// and position order no longer matches id order — it produces duplicates and
// gaps. With ~15 rows the explicit loop costs nothing.
func renumberItems(ctx context.Context, tx *sql.Tx, meetingID int64) error {
	ids, err := scanInt64s(ctx, tx,
		`SELECT id FROM meeting_items WHERE meeting_id = ? ORDER BY position, id`, meetingID)
	if err != nil {
		return fmt.Errorf("read agenda of meeting %d: %w", meetingID, err)
	}
	for pos, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE meeting_items SET position = ? WHERE id = ?`, pos, id); err != nil {
			return fmt.Errorf("renumber agenda item %d: %w", id, err)
		}
	}
	return nil
}

// ListItems returns a meeting's agenda in position order.
func (s *Store) ListItems(ctx context.Context, meetingID int64) ([]MeetingItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+itemColumns+` FROM meeting_items WHERE meeting_id = ? ORDER BY position, id`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("list agenda items: %w", err)
	}
	defer rows.Close()
	out := []MeetingItem{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agenda item: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddItems appends problems to a meeting's agenda, in the order given.
//
// Every check runs before any insert, so a rejected request writes nothing:
// an unknown problem id is ErrUnknownProblem, one already on this agenda is
// ErrConflict. Duplicate ids within the request are the caller's to reject.
func (s *Store) AddItems(ctx context.Context, meetingSlug string, problemIDs []int64, now time.Time) ([]MeetingItem, error) {
	ts := rfc3339(now)
	var created []MeetingItem
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var meetingID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM meetings WHERE slug = ?`, meetingSlug).Scan(&meetingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up meeting %q: %w", meetingSlug, err)
		}

		for _, pid := range problemIDs {
			var exists int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM problems WHERE id = ?`, pid).Scan(&exists)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %d", ErrUnknownProblem, pid)
			}
			if err != nil {
				return fmt.Errorf("look up problem %d: %w", pid, err)
			}
			var onAgenda int
			err = tx.QueryRowContext(ctx,
				`SELECT 1 FROM meeting_items WHERE meeting_id = ? AND problem_id = ?`, meetingID, pid).Scan(&onAgenda)
			if err == nil {
				return fmt.Errorf("%w: problem %d", ErrConflict, pid)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("check agenda for problem %d: %w", pid, err)
			}
		}

		var next int
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(position) + 1, 0) FROM meeting_items WHERE meeting_id = ?`, meetingID).Scan(&next); err != nil {
			return fmt.Errorf("find next agenda position: %w", err)
		}

		for _, pid := range problemIDs {
			res, err := tx.ExecContext(ctx,
				`INSERT INTO meeting_items (meeting_id, problem_id, position, created_at) VALUES (?, ?, ?, ?)`,
				meetingID, pid, next, ts)
			if err != nil {
				return fmt.Errorf("add problem %d to the agenda: %w", pid, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("read new agenda item id: %w", err)
			}
			created = append(created, MeetingItem{
				ID: id, MeetingID: meetingID, ProblemID: pid, Position: next, CreatedAt: ts,
			})
			next++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ReorderItems rewrites every position of one meeting from the given id list.
//
// The list must name this meeting's items exactly once each, with nothing
// missing and nothing foreign — anything else is ErrInvalidOrder and no
// position changes. A partial list would silently strand the omitted items.
func (s *Store) ReorderItems(ctx context.Context, meetingSlug string, itemIDs []int64) ([]MeetingItem, error) {
	var out []MeetingItem
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var meetingID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM meetings WHERE slug = ?`, meetingSlug).Scan(&meetingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("look up meeting %q: %w", meetingSlug, err)
		}

		current, err := scanInt64s(ctx, tx, `SELECT id FROM meeting_items WHERE meeting_id = ?`, meetingID)
		if err != nil {
			return fmt.Errorf("load agenda items: %w", err)
		}
		if len(current) != len(itemIDs) {
			return ErrInvalidOrder
		}
		want := make(map[int64]bool, len(current))
		for _, id := range current {
			want[id] = true
		}
		seen := make(map[int64]bool, len(itemIDs))
		for _, id := range itemIDs {
			if !want[id] || seen[id] {
				return ErrInvalidOrder
			}
			seen[id] = true
		}

		for pos, id := range itemIDs {
			if _, err := tx.ExecContext(ctx,
				`UPDATE meeting_items SET position = ? WHERE id = ? AND meeting_id = ?`, pos, id, meetingID); err != nil {
				return fmt.Errorf("set position of agenda item %d: %w", id, err)
			}
		}

		rows, err := tx.QueryContext(ctx,
			`SELECT `+itemColumns+` FROM meeting_items WHERE meeting_id = ? ORDER BY position, id`, meetingID)
		if err != nil {
			return fmt.Errorf("reload agenda: %w", err)
		}
		defer rows.Close()
		out = []MeetingItem{}
		for rows.Next() {
			it, err := scanItem(rows)
			if err != nil {
				return fmt.Errorf("scan agenda item: %w", err)
			}
			out = append(out, it)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateItem edits an agenda entry's notes.
//
// itemID is a global meeting_items id, so the update is scoped to the meeting
// named by meetingSlug and answers ErrNotFound on a mismatch. Looking the item
// up by id alone would let a request against one week write a note onto
// another week's agenda.
func (s *Store) UpdateItem(ctx context.Context, meetingSlug string, itemID int64, patch ItemPatch) (MeetingItem, error) {
	var out MeetingItem
	err := s.tx(ctx, func(tx *sql.Tx) error {
		meetingID, err := itemMeeting(ctx, tx, meetingSlug, itemID)
		if err != nil {
			return err
		}
		var sets []string
		var args []any
		if patch.PrepNote != nil {
			sets = append(sets, "prep_note = ?")
			args = append(args, *patch.PrepNote)
		}
		if patch.ActionNote != nil {
			sets = append(sets, "action_note = ?")
			args = append(args, *patch.ActionNote)
		}
		if len(sets) > 0 {
			args = append(args, itemID, meetingID)
			query := "UPDATE meeting_items SET " + joinComma(sets) + " WHERE id = ? AND meeting_id = ?"
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return fmt.Errorf("update agenda item %d: %w", itemID, err)
			}
		}
		out, err = scanItem(tx.QueryRowContext(ctx, `SELECT `+itemColumns+` FROM meeting_items WHERE id = ?`, itemID))
		if err != nil {
			return fmt.Errorf("reload agenda item %d: %w", itemID, err)
		}
		return nil
	})
	if err != nil {
		return MeetingItem{}, err
	}
	return out, nil
}

// DeleteItem removes an agenda entry and renumbers what is left of that
// meeting's agenda. The problem itself is untouched and stays in the bucket.
//
// Like UpdateItem, this is scoped to meetingSlug: deleting on a mismatched pair
// would remove one week's entry and then renumber a different week entirely.
func (s *Store) DeleteItem(ctx context.Context, meetingSlug string, itemID int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		meetingID, err := itemMeeting(ctx, tx, meetingSlug, itemID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM meeting_items WHERE id = ? AND meeting_id = ?`, itemID, meetingID); err != nil {
			return fmt.Errorf("remove agenda item %d: %w", itemID, err)
		}
		return renumberItems(ctx, tx, meetingID)
	})
}

// itemMeeting resolves the meeting id for a slug and verifies that itemID
// really is one of its agenda entries.
func itemMeeting(ctx context.Context, tx *sql.Tx, meetingSlug string, itemID int64) (int64, error) {
	var meetingID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM meetings WHERE slug = ?`, meetingSlug).Scan(&meetingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("look up meeting %q: %w", meetingSlug, err)
	}
	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT meeting_id FROM meeting_items WHERE id = ?`, itemID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("look up agenda item %d: %w", itemID, err)
	}
	if owner != meetingID {
		return 0, ErrNotFound
	}
	return meetingID, nil
}

// ProblemMeetingRefs returns a problem's cross-meeting timeline, oldest first.
// This is the payoff of storing the notes per agenda entry: each appearance
// keeps its own pair of notes instead of the latest week overwriting the last.
func (s *Store) ProblemMeetingRefs(ctx context.Context, problemID int64) ([]ProblemMeetingRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, m.id, m.slug, m.meeting_date, i.position, i.prep_note, i.action_note
		  FROM meeting_items i
		  JOIN meetings m ON m.id = i.meeting_id
		 WHERE i.problem_id = ?
		 ORDER BY m.meeting_date ASC, m.id ASC`, problemID)
	if err != nil {
		return nil, fmt.Errorf("list the problem's meetings: %w", err)
	}
	defer rows.Close()
	out := []ProblemMeetingRef{}
	for rows.Next() {
		var r ProblemMeetingRef
		if err := rows.Scan(&r.ItemID, &r.MeetingID, &r.Slug, &r.MeetingDate, &r.Position, &r.PrepNote, &r.ActionNote); err != nil {
			return nil, fmt.Errorf("scan meeting reference: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
