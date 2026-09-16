package search

import (
	"sync"
	"testing"
)

func TestFold(t *testing.T) {
	f := NewFolder()
	cases := []struct{ in, want string }{
		{"řešení", "reseni"},
		{"Řešení", "reseni"},
		{"Č", "c"},
		{"ěščřžýáíéúůťďň", "escrzyaieuutdn"},
		{"ĚŠČŘŽÝÁÍÉÚŮŤĎŇ", "escrzyaieuutdn"},
		{"Eva Dvořáková", "eva dvorakova"},
		{"plain ascii", "plain ascii"},
		{"", ""},
	}
	for _, c := range cases {
		if got := f.Fold(c.in); got != c.want {
			t.Errorf("Fold(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The folder must be reusable: a stale transformer state would corrupt the
// second and later calls.
func TestFoldIsRepeatable(t *testing.T) {
	f := NewFolder()
	for i := 0; i < 100; i++ {
		if got := f.Fold("Náhled rozdílů nezobrazuje diakritiku správně"); got != "nahled rozdilu nezobrazuje diakritiku spravne" {
			t.Fatalf("call %d returned %q", i, got)
		}
	}
}

// Decomposed input must match precomposed storage and vice versa.
func TestFoldNormalisesBothForms(t *testing.T) {
	f := NewFolder()
	precomposed := "\u0159e\u0161en\u00ed"        // řešení
	decomposed := "r\u030ces\u030cen\u0069\u0301" // r+caron, s+caron, i+acute
	if a, b := f.Fold(precomposed), f.Fold(decomposed); a != b {
		t.Fatalf("NFC %q folded to %q but NFD folded to %q", precomposed, a, b)
	}
}

type row struct{ title, desc string }

func TestFilter(t *testing.T) {
	rows := []row{
		{"Náhled rozdílů nezobrazuje diakritiku správně", "Chrome i Firefox"},
		{"Import nad 5 000 řádků spadne na timeout", "dávkový soubor zůstane ve frontě"},
		{"Tlačítko „Vyřešit vše“ nejde zrušit", ""},
	}
	text := func(r row) []string { return []string{r.title, r.desc} }

	// Unaccented query finds accented text.
	if got := Filter(rows, "rozdilu", text); len(got) != 1 || got[0].title != rows[0].title {
		t.Errorf("q=rozdilu matched %d rows, want the first", len(got))
	}
	// The description is searched too.
	if got := Filter(rows, "davkovy", text); len(got) != 1 || got[0].title != rows[1].title {
		t.Errorf("q=davkovy matched %d rows, want the second", len(got))
	}
	// Case-insensitive.
	if got := Filter(rows, "VYRESIT", text); len(got) != 1 {
		t.Errorf("q=VYRESIT matched %d rows, want 1", len(got))
	}
	// Empty and whitespace-only queries are a no-op that preserves order.
	for _, q := range []string{"", "   "} {
		got := Filter(rows, q, text)
		if len(got) != len(rows) || got[0].title != rows[0].title {
			t.Errorf("q=%q should return every row in order, got %d", q, len(got))
		}
	}
	// No match.
	if got := Filter(rows, "kubernetes", text); len(got) != 0 {
		t.Errorf("q=kubernetes matched %d rows, want 0", len(got))
	}
}

// Each call must build its own folder; this fails under -race if a transformer
// is ever shared at package level.
func TestFilterIsConcurrencySafe(t *testing.T) {
	t.Parallel()
	rows := []row{{"řešení konfliktu", ""}, {"import", ""}}
	text := func(r row) []string { return []string{r.title, r.desc} }
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if got := Filter(rows, "reseni", text); len(got) != 1 {
					t.Errorf("concurrent Filter returned %d rows, want 1", len(got))
					return
				}
			}
		}()
	}
	wg.Wait()
}
