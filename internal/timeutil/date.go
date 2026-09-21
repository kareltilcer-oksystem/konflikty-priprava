// Package timeutil holds the few date helpers the app needs. Dates on the wire
// and in SQLite are plain YYYY-MM-DD; timestamps are RFC3339 in UTC.
package timeutil

import (
	"fmt"
	"time"
)

// DateLayout is the storage and wire format for a plain date (meetings.meeting_date).
const DateLayout = "2006-01-02"

// MinYear and MaxYear bound meeting_date. The slug's published pattern starts
// with [0-9]{4}, so a date outside this range would generate a slug that does
// not match the contract the URL is validated against.
const (
	MinYear = 2000
	MaxYear = 2099
)

// ParseDate reads a YYYY-MM-DD date as a UTC midnight time and rejects anything
// outside MinYear..MaxYear. time.Parse with DateLayout is already strict about
// the shape, so "2026-9-1" and "2026-02-31" are both refused.
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(DateLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q is not YYYY-MM-DD", s)
	}
	if y := t.Year(); y < MinYear || y > MaxYear {
		return time.Time{}, fmt.Errorf("date %q is outside %d-%d", s, MinYear, MaxYear)
	}
	return t, nil
}

// FormatDate renders a date in the storage format.
func FormatDate(t time.Time) string { return t.Format(DateLayout) }

// FormatTimestamp renders an instant as RFC3339 in UTC, the format every
// created_at / updated_at / expires_at column uses.
func FormatTimestamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// Today is the current date at UTC midnight, in the server's local day. The
// archive cutoff compares dates, not instants, so the local calendar day is the
// right reference: a meeting "today" must never count as archived.
func Today(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ArchiveCutoff is the oldest meeting_date that still counts as current.
// A meeting is archived when its date is strictly before this.
func ArchiveCutoff(now time.Time, archiveAfterDays int) time.Time {
	return Today(now).AddDate(0, 0, -archiveAfterDays)
}

// IsArchived reports whether a meeting date falls outside the archive window.
// Future dates are never archived — the meeting being prepared for the coming
// week is always future-dated and is the one the list exists to surface.
func IsArchived(meetingDate time.Time, now time.Time, archiveAfterDays int) bool {
	return meetingDate.Before(ArchiveCutoff(now, archiveAfterDays))
}
