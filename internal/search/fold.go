// Package search implements the diacritics- and case-insensitive filter behind
// the bucket's `q` parameter.
//
// This deliberately does not happen in SQL. SQLite's LIKE and NOCASE fold ASCII
// only and modernc.org/sqlite carries no ICU, so neither can match `reseni`
// against `řešení`. The candidate set is tiny by design (low hundreds of rows
// over years), so the handler loads it and filters here.
package search

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Folder normalises text for comparison: NFD, drop combining marks, lower-case.
//
// It is NOT safe for concurrent use. transform.Transformer carries state
// between calls, so a package-level chain shared across requests would corrupt
// its own output under load. Construct one per call site and reuse it within
// that call — which is what Filter does across the rows of a single request.
type Folder struct {
	t transform.Transformer
}

// NewFolder builds a folder. Runes without a canonical decomposition — ł, đ, ø
// — are unaffected by NFD and so do not fold to ASCII. That matches the
// specified behaviour and is irrelevant for Czech.
func NewFolder() *Folder {
	return &Folder{
		t: transform.Chain(
			norm.NFD,
			runes.Remove(runes.In(unicode.Mn)),
			norm.NFC,
		),
	}
}

// Fold returns the comparison form of s. On the (practically unreachable)
// transform error it falls back to a plain lower-casing rather than dropping
// the row from the result.
func (f *Folder) Fold(s string) string {
	f.t.Reset()
	out, _, err := transform.String(f.t, s)
	if err != nil {
		return strings.ToLower(s)
	}
	return strings.ToLower(out)
}

// Matches reports whether any of the haystacks contains the already-folded
// needle.
func (f *Folder) Matches(needle string, haystacks ...string) bool {
	for _, h := range haystacks {
		if strings.Contains(f.Fold(h), needle) {
			return true
		}
	}
	return false
}

// Filter keeps the items whose text fields contain q, comparing both sides in
// folded form. An empty or whitespace-only q returns the input untouched, and
// the input order is always preserved — the SQL layer has already applied the
// requested sort.
func Filter[T any](items []T, q string, text func(T) []string) []T {
	q = strings.TrimSpace(q)
	if q == "" {
		return items
	}
	f := NewFolder()
	needle := f.Fold(q)
	out := make([]T, 0, len(items))
	for _, it := range items {
		if f.Matches(needle, text(it)...) {
			out = append(out, it)
		}
	}
	return out
}
