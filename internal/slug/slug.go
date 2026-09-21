// Package slug derives the readable meeting identifier used in /porada/{slug}.
package slug

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Pattern is a verbatim copy of the MeetingSlug pattern in api/openapi.yaml.
// It exists so tests can assert that every slug we generate is reachable at its
// own URL. The suffix alternative is spelled [2-9]|[1-9][0-9]+ on purpose:
// -[2-9][0-9]* would accept -9 and -20 while rejecting -10.
var Pattern = regexp.MustCompile(`^[0-9]{4}-w(0[1-9]|[1-4][0-9]|5[0-3])(-([2-9]|[1-9][0-9]+))?$`)

// Base is the slug for the first meeting of a date's ISO week, e.g. 2026-w38.
//
// Both halves come from the same ISOWeek call. The year is the ISO
// week-numbering year, not the calendar year: they disagree at every year
// boundary. 2027-01-01 is ISO 2026-W53, and 2025-12-29 is ISO 2026-W01.
// Pairing the calendar year with the ISO week number instead would emit
// 2027-w53 for a year with no 53rd week, and would collide 2025-12-29 with the
// meeting of 2025-01-02.
func Base(d time.Time) string {
	year, week := d.ISOWeek()
	return fmt.Sprintf("%04d-w%02d", year, week)
}

// WithSuffix returns the nth slug for an ISO week. The first meeting of a week
// carries no suffix; an extra one starts at -2 and continues -3, -10, -11, ...
func WithSuffix(base string, n int) string {
	if n <= 1 {
		return base
	}
	return base + "-" + strconv.Itoa(n)
}

// ISOWeek returns the ISO week-numbering year and week of a date — the same
// pair Base is built from, used for the derived iso_year / iso_week fields.
func ISOWeek(d time.Time) (year, week int) { return d.ISOWeek() }
