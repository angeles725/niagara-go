package scope

import (
	"fmt"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

func hits(sources ...string) []*fts.Hit {
	out := make([]*fts.Hit, len(sources))
	for i, s := range sources {
		out[i] = &fts.Hit{Source: s, Path: fmt.Sprintf("p%d.txt", i), Title: fmt.Sprintf("T%d", i)}
	}
	return out
}

func row(h *fts.Hit, tag string) string {
	return "  [" + tag + "] " + h.Title + "\n    path: " + h.Key() + "\n"
}

// FindReport keeps at most perSource hits per source, orders the totals by
// count (ties in first-seen order) and names the hits it left out.
func TestFindReportCapsPerSourceAndTotals(t *testing.T) {
	r := &fts.Result{Hits: hits("b", "a", "a", "c", "a"), ModeUsed: "and"}
	h := r.Hits
	want := "5 files matching 'q' (all terms):\nby source: a 3, b 1, c 1\n\n" +
		row(h[0], "b") + "\n" + row(h[1], "a") + "\n" + row(h[3], "c") + "\n" +
		"  ... and 2 more files\n"
	if got := FindReport(t.TempDir(), "q", r, 1); got != want {
		t.Errorf("FindReport =\n%q\nwant\n%q", got, want)
	}
}

// FindReport never shows more than findMaxResults hits.
func TestFindReportCapsAtTen(t *testing.T) {
	src := make([]string, findMaxResults+2)
	for i := range src {
		src[i] = "a"
	}
	r := &fts.Result{Hits: hits(src...), ModeUsed: "or"}
	got := FindReport(t.TempDir(), "q", r, 99)
	if want := "  ... and 2 more files\n"; got[len(got)-len(want):] != want {
		t.Errorf("FindReport tail = %q, want %q", got[len(got)-len(want):], want)
	}
}

// RankedReport's "(how)" names the mode and a fallback; an empty totals
// prints no totals line, and every row carries its tag.
func TestRankedReportHowAndTotals(t *testing.T) {
	tag := func(h *fts.Hit) string { return "x-" + h.Source }
	for _, c := range []struct {
		r   fts.Result
		how string
	}{
		{fts.Result{ModeUsed: "and"}, "all terms"},
		{fts.Result{ModeUsed: "or"}, "any term, ranked by how many terms match"},
		{fts.Result{ModeUsed: "or", Fallback: true}, "no file has every term; showing files with any term, ranked by how many terms match"},
		{fts.Result{ModeUsed: "or", Fallback: true, AndHits: 1}, "only 1 file has every term; showing files with any term, ranked by how many terms match"},
		{fts.Result{ModeUsed: "and", Fallback: true, AndHits: 2}, "only 2 files have every term; showing files with any term, ranked by how many terms match"},
	} {
		c.r.Hits = hits("s")
		want := "1 guide files matching 'q' (" + c.how + "):\n\n" + row(c.r.Hits[0], "x-s") + "\n"
		if got := RankedReport(t.TempDir(), "q", &c.r, c.r.Hits, "guide files", "", tag); got != want {
			t.Errorf("RankedReport(%+v) =\n%q\nwant\n%q", c.r, got, want)
		}
	}
}
