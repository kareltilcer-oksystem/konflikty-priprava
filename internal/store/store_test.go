package store

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// newStore opens a throwaway database.
//
// t.TempDir() is called first and Close is registered as a cleanup, so the
// handle is released before the directory is removed — on Windows a still-open
// file makes RemoveAll fail, and a failed cleanup fails the test.
func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func at(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

var now = time.Date(2026, 9, 16, 8, 30, 0, 0, time.UTC)

func mustProblem(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	id, err := s.CreateProblem(context.Background(), ProblemInput{Title: title}, nil, "jan", now)
	if err != nil {
		t.Fatalf("CreateProblem(%q): %v", title, err)
	}
	return id
}

func mustMeeting(t *testing.T, s *Store, date string) Meeting {
	t.Helper()
	m, err := s.CreateMeeting(context.Background(), at(t, date), "", "admin", now)
	if err != nil {
		t.Fatalf("CreateMeeting(%s): %v", date, err)
	}
	return m
}

func positions(t *testing.T, s *Store, meetingID int64) []int {
	t.Helper()
	items, err := s.ListItems(context.Background(), meetingID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = it.Position
	}
	return out
}

func equalInts(a []int, b ...int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A DSN typo would silently drop either pragma, and nothing else would notice
// until a cascade failed to fire or a concurrent reader blocked.
func TestOpenSetsPragmas(t *testing.T) {
	s := newStore(t)
	var fk int
	if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
	var mode string
	if err := s.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := s1.CreateProblem(context.Background(), ProblemInput{Title: "x"}, nil, "jan", now); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()
	ps, err := s2.ListProblems(context.Background(), ProblemFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Errorf("reopening the database lost data: %d problems, want 1", len(ps))
	}
}

// Deleting from the middle must leave the survivors contiguous, or the agenda
// renders as 1, 3, 4.
func TestDeleteItemRenumbers(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, mustProblem(t, s, "problem"))
	}
	items, err := s.AddItems(ctx, m.Slug, ids, now)
	if err != nil {
		t.Fatalf("AddItems: %v", err)
	}
	if got := positions(t, s, m.ID); !equalInts(got, 0, 1, 2, 3, 4) {
		t.Fatalf("initial positions %v", got)
	}

	if err := s.DeleteItem(ctx, m.Slug, items[2].ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if got := positions(t, s, m.ID); !equalInts(got, 0, 1, 2, 3) {
		t.Errorf("positions after delete = %v, want 0 1 2 3", got)
	}
	// The order of the survivors must be preserved.
	left, _ := s.ListItems(ctx, m.ID)
	want := []int64{items[0].ID, items[1].ID, items[3].ID, items[4].ID}
	for i, it := range left {
		if it.ID != want[i] {
			t.Errorf("item %d is %d, want %d", i, it.ID, want[i])
		}
	}
}

// Regression: renumbering must survive an agenda whose position order no
// longer matches its id order.
//
// A correlated-subquery renumber passes the simple case above and fails here,
// producing [0 2 2] — a duplicate position and a gap — because SQLite walks the
// table in rowid order and the subquery sees rows the same statement has
// already rewritten.
func TestRenumberAfterReorder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")
	var pids []int64
	for i := 0; i < 4; i++ {
		pids = append(pids, mustProblem(t, s, "p"))
	}
	items, err := s.AddItems(ctx, m.Slug, pids, now)
	if err != nil {
		t.Fatal(err)
	}
	order := []int64{items[3].ID, items[0].ID, items[2].ID, items[1].ID}
	if _, err := s.ReorderItems(ctx, m.Slug, order); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteItem(ctx, m.Slug, order[1]); err != nil {
		t.Fatal(err)
	}
	if got := positions(t, s, m.ID); !equalInts(got, 0, 1, 2) {
		t.Fatalf("positions = %v, want 0 1 2 — the agenda would render with a gap", got)
	}
	// The surviving order must still be the one the admin dragged into place.
	left, _ := s.ListItems(ctx, m.ID)
	want := []int64{order[0], order[2], order[3]}
	for i, it := range left {
		if it.ID != want[i] {
			t.Errorf("slot %d holds item %d, want %d", i, it.ID, want[i])
		}
	}
}

// Deleting a problem strips it from every agenda it was on, and each of those
// agendas must be renumbered — while an unrelated meeting is left alone.
func TestDeleteProblemRenumbersEveryAffectedMeeting(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m1 := mustMeeting(t, s, "2026-09-04")
	m2 := mustMeeting(t, s, "2026-09-11")
	m3 := mustMeeting(t, s, "2026-09-18")

	a, b, c := mustProblem(t, s, "a"), mustProblem(t, s, "b"), mustProblem(t, s, "c")
	if _, err := s.AddItems(ctx, m1.Slug, []int64{a, b, c}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddItems(ctx, m2.Slug, []int64{c, a}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddItems(ctx, m3.Slug, []int64{b, c}, now); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteProblem(ctx, a); err != nil {
		t.Fatalf("DeleteProblem: %v", err)
	}
	if got := positions(t, s, m1.ID); !equalInts(got, 0, 1) {
		t.Errorf("meeting 1 positions = %v, want 0 1", got)
	}
	if got := positions(t, s, m2.ID); !equalInts(got, 0) {
		t.Errorf("meeting 2 positions = %v, want 0", got)
	}
	if got := positions(t, s, m3.ID); !equalInts(got, 0, 1) {
		t.Errorf("meeting 3 (untouched) positions = %v, want 0 1", got)
	}
}

// DeleteProblem must hand back the files to unlink; SQLite's cascade removes
// the rows but cannot remove the bytes.
func TestDeleteProblemReturnsStorageNames(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	id, err := s.CreateProblem(ctx, ProblemInput{Title: "with files"}, []NewAttachment{
		{Filename: "a.png", ContentType: "image/png", SizeBytes: 10, StorageName: "uuid-a.png"},
		{Filename: "b.log", ContentType: "text/plain", SizeBytes: 20, StorageName: "uuid-b.txt"},
	}, "jan", now)
	if err != nil {
		t.Fatal(err)
	}
	names, err := s.DeleteProblem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d storage names, want 2: %v", len(names), names)
	}
}

// An item id is global, so both note edits and removals must be scoped to the
// meeting in the path — otherwise a request against one week mutates another.
func TestItemOperationsAreScopedToTheirMeeting(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	week31 := mustMeeting(t, s, "2026-07-31")
	week38 := mustMeeting(t, s, "2026-09-18")

	foreign, err := s.AddItems(ctx, week31.Slug, []int64{mustProblem(t, s, "foreign")}, now)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for i := 0; i < 3; i++ {
		ids = append(ids, mustProblem(t, s, "own"))
	}
	if _, err := s.AddItems(ctx, week38.Slug, ids, now); err != nil {
		t.Fatal(err)
	}

	// Deleting week 31's item through week 38's URL must 404 and renumber nothing.
	if err := s.DeleteItem(ctx, week38.Slug, foreign[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteItem across meetings = %v, want ErrNotFound", err)
	}
	if got := positions(t, s, week31.ID); !equalInts(got, 0) {
		t.Errorf("week 31 was modified: %v", got)
	}
	if got := positions(t, s, week38.ID); !equalInts(got, 0, 1, 2) {
		t.Errorf("week 38 was renumbered by a rejected request: %v", got)
	}

	// The same shape on a note edit must not write onto another week's item.
	note := "nope"
	if _, err := s.UpdateItem(ctx, week38.Slug, foreign[0].ID, ItemPatch{PrepNote: &note}); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateItem across meetings = %v, want ErrNotFound", err)
	}
	got, _ := s.ListItems(ctx, week31.ID)
	if got[0].PrepNote != "" {
		t.Errorf("week 31's note was overwritten: %q", got[0].PrepNote)
	}
}

func TestReorderItems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")
	var ids []int64
	for i := 0; i < 4; i++ {
		ids = append(ids, mustProblem(t, s, "p"))
	}
	items, err := s.AddItems(ctx, m.Slug, ids, now)
	if err != nil {
		t.Fatal(err)
	}
	order := []int64{items[3].ID, items[0].ID, items[2].ID, items[1].ID}
	got, err := s.ReorderItems(ctx, m.Slug, order)
	if err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}
	for i, it := range got {
		if it.ID != order[i] || it.Position != i {
			t.Errorf("position %d holds item %d at %d, want item %d", i, it.ID, it.Position, order[i])
		}
	}

	// A partial, duplicated or foreign list must change nothing.
	bad := map[string][]int64{
		"partial":   {items[0].ID, items[1].ID},
		"duplicate": {items[0].ID, items[0].ID, items[1].ID, items[2].ID},
		"foreign":   {items[0].ID, items[1].ID, items[2].ID, 9999},
	}
	for name, list := range bad {
		if _, err := s.ReorderItems(ctx, m.Slug, list); !errors.Is(err, ErrInvalidOrder) {
			t.Errorf("%s order = %v, want ErrInvalidOrder", name, err)
		}
		after, _ := s.ListItems(ctx, m.ID)
		for i, it := range after {
			if it.ID != order[i] {
				t.Fatalf("%s order changed the agenda", name)
			}
		}
	}
}

// Three meetings in one ISO week get base, -2, -3; a different week is
// unaffected.
func TestCreateMeetingAllocatesSlugSuffixes(t *testing.T) {
	s := newStore(t)
	a := mustMeeting(t, s, "2026-09-18")
	b := mustMeeting(t, s, "2026-09-17")
	c := mustMeeting(t, s, "2026-09-16")
	if a.Slug != "2026-w38" || b.Slug != "2026-w38-2" || c.Slug != "2026-w38-3" {
		t.Errorf("slugs = %q, %q, %q; want 2026-w38, 2026-w38-2, 2026-w38-3", a.Slug, b.Slug, c.Slug)
	}
	if d := mustMeeting(t, s, "2026-09-25"); d.Slug != "2026-w39" {
		t.Errorf("next week's slug = %q, want 2026-w39", d.Slug)
	}
}

// Changing the date moves the week label but never the URL.
func TestUpdateMeetingKeepsTheSlug(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")
	newDate := "2026-09-25"
	got, err := s.UpdateMeeting(ctx, m.Slug, MeetingPatch{MeetingDate: &newDate})
	if err != nil {
		t.Fatalf("UpdateMeeting: %v", err)
	}
	if got.Slug != "2026-w38" {
		t.Errorf("slug changed to %q; shared links would break", got.Slug)
	}
	if got.MeetingDate != newDate {
		t.Errorf("meeting_date = %q, want %q", got.MeetingDate, newDate)
	}
}

func TestAddItemsRejectsDuplicatesAndUnknownProblems(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")
	p := mustProblem(t, s, "p")

	if _, err := s.AddItems(ctx, m.Slug, []int64{p}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddItems(ctx, m.Slug, []int64{p}, now); !errors.Is(err, ErrConflict) {
		t.Errorf("re-adding = %v, want ErrConflict", err)
	}
	// A rejected batch must write nothing at all.
	other := mustProblem(t, s, "other")
	if _, err := s.AddItems(ctx, m.Slug, []int64{other, p}, now); !errors.Is(err, ErrConflict) {
		t.Errorf("batch with a duplicate = %v, want ErrConflict", err)
	}
	if got := positions(t, s, m.ID); !equalInts(got, 0) {
		t.Errorf("a rejected batch was partially applied: %v", got)
	}
	if _, err := s.AddItems(ctx, m.Slug, []int64{9999}, now); !errors.Is(err, ErrUnknownProblem) {
		t.Errorf("unknown problem = %v, want ErrUnknownProblem", err)
	}
}

// Marking an already-done problem done again must not rewrite done_at/done_by.
func TestSetProblemDoneIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	id := mustProblem(t, s, "p")

	if err := s.SetProblemDone(ctx, id, true, "admin", now); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetProblem(ctx, id)
	if !first.Done || first.DoneAt == nil || first.DoneBy == nil || *first.DoneBy != "admin" {
		t.Fatalf("after marking done: %+v", first)
	}

	later := now.Add(48 * time.Hour)
	if err := s.SetProblemDone(ctx, id, true, "eva", later); err != nil {
		t.Fatal(err)
	}
	again, _ := s.GetProblem(ctx, id)
	if *again.DoneAt != *first.DoneAt || *again.DoneBy != *first.DoneBy {
		t.Errorf("a repeated done rewrote history: %v/%v became %v/%v",
			*first.DoneAt, *first.DoneBy, *again.DoneAt, *again.DoneBy)
	}

	if err := s.SetProblemDone(ctx, id, false, "admin", later); err != nil {
		t.Fatal(err)
	}
	reopened, _ := s.GetProblem(ctx, id)
	if reopened.Done || reopened.DoneAt != nil || reopened.DoneBy != nil {
		t.Errorf("after reopening: %+v", reopened)
	}
}

func TestListProblemsFiltersAndSorts(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	m := mustMeeting(t, s, "2026-09-18")

	oldest, err := s.CreateProblem(ctx, ProblemInput{Title: "oldest"}, nil, "jan", now.Add(-72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	middle, err := s.CreateProblem(ctx, ProblemInput{Title: "middle"}, nil, "eva", now.Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	newest, err := s.CreateProblem(ctx, ProblemInput{Title: "newest"}, nil, "jan", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddItems(ctx, m.Slug, []int64{middle}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProblemDone(ctx, oldest, true, "admin", now); err != nil {
		t.Fatal(err)
	}

	notDone := false
	open, err := s.ListProblems(ctx, ProblemFilter{Done: &notDone})
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 2 {
		t.Errorf("the bucket holds %d problems, want 2", len(open))
	}
	if open[0].ID != newest {
		t.Errorf("default sort put %d first, want the newest (%d)", open[0].ID, newest)
	}

	asc, _ := s.ListProblems(ctx, ProblemFilter{Sort: SortCreatedAsc})
	if asc[0].ID != oldest {
		t.Errorf("created_asc put %d first, want %d", asc[0].ID, oldest)
	}

	byMeetings, _ := s.ListProblems(ctx, ProblemFilter{Sort: SortMeetingsDesc})
	if byMeetings[0].ID != middle {
		t.Errorf("meetings_desc put %d first, want the scheduled problem (%d)", byMeetings[0].ID, middle)
	}

	yes := true
	scheduled, _ := s.ListProblems(ctx, ProblemFilter{Scheduled: &yes})
	if len(scheduled) != 1 || scheduled[0].ID != middle {
		t.Errorf("scheduled=true returned %d rows, want just %d", len(scheduled), middle)
	}

	// The counts the bucket badge reads.
	one, _ := s.GetProblem(ctx, middle)
	if one.MeetingCount != 1 || one.AttachmentCount != 0 {
		t.Errorf("counts = %d meetings / %d attachments, want 1 / 0", one.MeetingCount, one.AttachmentCount)
	}
}

func TestProblemMeetingRefsAreOldestFirst(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	p := mustProblem(t, s, "recurring")
	late := mustMeeting(t, s, "2026-09-18")
	early := mustMeeting(t, s, "2026-08-21")

	if _, err := s.AddItems(ctx, late.Slug, []int64{p}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddItems(ctx, early.Slug, []int64{p}, now); err != nil {
		t.Fatal(err)
	}
	refs, err := s.ProblemMeetingRefs(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("got %d timeline entries, want 2", len(refs))
	}
	if refs[0].Slug != early.Slug || refs[1].Slug != late.Slug {
		t.Errorf("timeline order = %q, %q; want oldest first", refs[0].Slug, refs[1].Slug)
	}
}

// Each appearance keeps its own notes — the whole reason they live on the
// agenda entry rather than on the problem.
func TestNotesAreStoredPerAppearance(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	p := mustProblem(t, s, "recurring")
	w37 := mustMeeting(t, s, "2026-09-11")
	w38 := mustMeeting(t, s, "2026-09-18")

	a, _ := s.AddItems(ctx, w37.Slug, []int64{p}, now)
	b, _ := s.AddItems(ctx, w38.Slug, []int64{p}, now)

	first, second := "poprve", "podruhe"
	if _, err := s.UpdateItem(ctx, w37.Slug, a[0].ID, ItemPatch{PrepNote: &first}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateItem(ctx, w38.Slug, b[0].ID, ItemPatch{PrepNote: &second}); err != nil {
		t.Fatal(err)
	}
	refs, _ := s.ProblemMeetingRefs(ctx, p)
	if refs[0].PrepNote != first || refs[1].PrepNote != second {
		t.Errorf("notes = %q / %q, want %q / %q", refs[0].PrepNote, refs[1].PrepNote, first, second)
	}
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	expires := now.Add(720 * time.Hour)
	if err := s.CreateSession(ctx, "tok", "jan", now, expires); err != nil {
		t.Fatal(err)
	}

	later := now.Add(time.Hour)
	user, err := s.SlideSession(ctx, "tok", later, later.Add(720*time.Hour))
	if err != nil {
		t.Fatalf("SlideSession: %v", err)
	}
	if user != "jan" {
		t.Errorf("session belongs to %q, want jan", user)
	}

	// Using it must have moved the expiry.
	var got string
	if err := s.DB().QueryRow(`SELECT expires_at FROM sessions WHERE token = 'tok'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == rfc3339(expires) {
		t.Error("expires_at did not move; the session is not sliding")
	}

	if _, err := s.SlideSession(ctx, "nope", later, later); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token = %v, want ErrNotFound", err)
	}

	// An expired row is anonymous, and the purge removes it.
	if err := s.CreateSession(ctx, "old", "eva", now.Add(-800*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SlideSession(ctx, "old", now, now.Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired token = %v, want ErrNotFound", err)
	}
	n, err := s.PurgeExpiredSessions(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("purged %d sessions, want 1", n)
	}

	if err := s.DeleteSession(ctx, "tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SlideSession(ctx, "tok", later, later); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted token = %v, want ErrNotFound", err)
	}
	// Logging out without a session is a no-op.
	if err := s.DeleteSession(ctx, "never-existed"); err != nil {
		t.Errorf("deleting an absent session should be a no-op, got %v", err)
	}
}

func TestNotFoundErrors(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.GetProblem(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProblem = %v, want ErrNotFound", err)
	}
	if _, err := s.GetMeetingBySlug(ctx, "2026-w12"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetMeetingBySlug = %v, want ErrNotFound", err)
	}
	if _, err := s.GetAttachment(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetAttachment = %v, want ErrNotFound", err)
	}
	if err := s.DeleteMeeting(ctx, "2026-w12"); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteMeeting = %v, want ErrNotFound", err)
	}
	if _, err := s.DeleteProblem(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteProblem = %v, want ErrNotFound", err)
	}
}

// Labels are a set: stored in canonical order whatever order they arrive in,
// replaced wholesale by a patch, and left alone by a patch that omits them.
func TestProblemLabels(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	id, err := s.CreateProblem(ctx, ProblemInput{
		Title: "štítkovaný", Labels: []string{"analysis", "ux"},
	}, nil, "jan", now)
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}
	p, err := s.GetProblem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Labels, []string{"ux", "analysis"}) {
		t.Errorf("labels = %v, want the canonical [ux analysis]", p.Labels)
	}

	// An omitted set survives an edit of the other fields.
	title := "přejmenovaný"
	if err := s.UpdateProblem(ctx, id, ProblemPatch{Title: &title}, nil, "jan", now); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetProblem(ctx, id); !slices.Equal(p.Labels, []string{"ux", "analysis"}) {
		t.Errorf("labels = %v after an unrelated patch, want them untouched", p.Labels)
	}

	// A given set replaces the whole set rather than adding to it.
	only := []string{"analysis"}
	if err := s.UpdateProblem(ctx, id, ProblemPatch{Labels: &only}, nil, "jan", now); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetProblem(ctx, id); !slices.Equal(p.Labels, []string{"analysis"}) {
		t.Errorf("labels = %v, want only [analysis]", p.Labels)
	}

	// The empty set clears them, and the list view agrees with the detail.
	none := []string{}
	if err := s.UpdateProblem(ctx, id, ProblemPatch{Labels: &none}, nil, "jan", now); err != nil {
		t.Fatal(err)
	}
	ps, err := s.ListProblems(ctx, ProblemFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("listed %d problems, want 1", len(ps))
	}
	if len(ps[0].Labels) != 0 {
		t.Errorf("labels = %v after clearing, want none", ps[0].Labels)
	}
}

// The cascade has to take the label rows with the problem: nothing else ever
// deletes them, and the ids are reused by AUTOINCREMENT's successor at most
// never — but an orphan row would still be a lie in the table.
func TestDeleteProblemRemovesLabels(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	id, err := s.CreateProblem(ctx, ProblemInput{Title: "x", Labels: []string{"ux"}}, nil, "jan", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteProblem(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM problem_labels`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d label rows survived the problem", n)
	}
}

func TestNormalizeLabels(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
		ok   bool
	}{
		{"none", nil, []string{}, true},
		{"one", []string{"ux"}, []string{"ux"}, true},
		{"reordered", []string{"analysis", "ux"}, []string{"ux", "analysis"}, true},
		{"duplicated", []string{"ux", "ux"}, []string{"ux"}, true},
		{"padded", []string{" ux "}, nil, false},
		{"unknown", []string{"ux", "backend"}, nil, false},
		{"wrong case", []string{"UX"}, nil, false},
		{"empty value", []string{""}, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NormalizeLabels(c.in)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && !slices.Equal(got, c.want) {
				t.Errorf("= %v, want %v", got, c.want)
			}
		})
	}
}

// The store normalises a set itself instead of trusting its caller: duplicates
// collapse rather than breaking the primary key, and an unknown value is refused
// rather than written and then hidden on every read.
func TestCreateProblemNormalisesLabels(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	id, err := s.CreateProblem(ctx, ProblemInput{Title: "x", Labels: []string{"ux", "ux"}}, nil, "jan", now)
	if err != nil {
		t.Fatalf("CreateProblem with a duplicate: %v", err)
	}
	if p, _ := s.GetProblem(ctx, id); !slices.Equal(p.Labels, []string{"ux"}) {
		t.Errorf("labels = %v, want [ux]", p.Labels)
	}
	if _, err := s.CreateProblem(ctx, ProblemInput{Title: "y", Labels: []string{"UX"}}, nil, "jan", now); err == nil {
		t.Error("CreateProblem accepted a label outside the vocabulary")
	}
}
