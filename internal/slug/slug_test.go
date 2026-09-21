package slug

import (
	"testing"
	"time"
)

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

// The year-boundary cases are the whole reason both halves of the slug must
// come from one ISOWeek call.
func TestBaseYearBoundaries(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-09-18", "2026-w38"}, // the sample meeting
		{"2027-01-01", "2026-w53"}, // ISO year trails the calendar year
		{"2025-12-29", "2026-w01"}, // ISO year leads the calendar year
		{"2026-01-01", "2026-w01"},
		{"2021-01-01", "2020-w53"},
		{"2019-12-30", "2020-w01"},
		{"2024-12-30", "2025-w01"},
		{"2026-02-02", "2026-w06"}, // zero padding
		{"2026-01-05", "2026-w02"},
	}
	for _, c := range cases {
		if got := Base(date(t, c.in)); got != c.want {
			t.Errorf("Base(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

// A calendar-year slug would collide these two, a year apart, on 2025-w01.
func TestBaseDoesNotCollideAcrossYears(t *testing.T) {
	a := Base(date(t, "2025-12-29"))
	b := Base(date(t, "2025-01-02"))
	if a == b {
		t.Fatalf("2025-12-29 and 2025-01-02 both produced %s", a)
	}
}

// Every slug we can generate must be reachable at its own URL, and every day of
// an ISO week must land on the same slug.
func TestBaseMatchesContractAndIsStableWithinAWeek(t *testing.T) {
	start := date(t, "2000-01-01")
	end := date(t, "2050-12-31")
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		got := Base(d)
		if !Pattern.MatchString(got) {
			t.Fatalf("Base(%s) = %s does not match the published pattern", d.Format("2006-01-02"), got)
		}
		// Monday of this ISO week must agree.
		offset := (int(d.Weekday()) + 6) % 7 // Monday = 0
		if monday := Base(d.AddDate(0, 0, -offset)); monday != got {
			t.Fatalf("%s gave %s but its Monday gave %s", d.Format("2006-01-02"), got, monday)
		}
	}
}

// -10 and above must match; this is what the regex alternative is spelled for.
func TestWithSuffixMatchesContract(t *testing.T) {
	const base = "2026-w38"
	if got := WithSuffix(base, 1); got != base {
		t.Errorf("WithSuffix(_, 1) = %s, want the bare base", got)
	}
	if got := WithSuffix(base, 0); got != base {
		t.Errorf("WithSuffix(_, 0) = %s, want the bare base", got)
	}
	for _, n := range []int{2, 3, 9, 10, 11, 20, 53, 100} {
		got := WithSuffix(base, n)
		if !Pattern.MatchString(got) {
			t.Errorf("WithSuffix(%s, %d) = %s does not match the published pattern", base, n, got)
		}
	}
}

func TestISOWeekAgreesWithBase(t *testing.T) {
	d := date(t, "2027-01-01")
	y, w := ISOWeek(d)
	if y != 2026 || w != 53 {
		t.Fatalf("ISOWeek(2027-01-01) = %d, %d; want 2026, 53", y, w)
	}
}
