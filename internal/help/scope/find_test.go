package scope

import (
	"fmt"
	"strings"
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

// FindCappedReport keeps at most perSource hits per source, orders the totals by
// count (ties in first-seen order) and names the hits it left out.
func TestFindReportCapsPerSourceAndTotals(t *testing.T) {
	r := &fts.Result{Hits: hits("b", "a", "a", "c", "a"), ModeUsed: "and"}
	h := r.Hits
	want := "5 files matching 'q' (all terms):\nby source: a 3, b 1, c 1\n\n" +
		row(h[0], "b") + "\n" + row(h[1], "a") + "\n" + row(h[3], "c") + "\n" +
		"  ... and 2 more files\n"
	if got := FindCappedReport(t.TempDir(), "q", r, 1); got != want {
		t.Errorf("FindReport =\n%q\nwant\n%q", got, want)
	}
}

// FindCappedReport never shows more than findMaxResults hits.
func TestFindReportCapsAtTen(t *testing.T) {
	src := make([]string, findMaxResults+2)
	for i := range src {
		src[i] = "a"
	}
	r := &fts.Result{Hits: hits(src...), ModeUsed: "or"}
	got := FindCappedReport(t.TempDir(), "q", r, 99)
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

// An explicit -n raises the total to n and gives every source with hits at
// least one slot, so a source the totals line counts is never invisible.
func TestFindReportRaisedShowsEverySource(t *testing.T) {
	src := make([]string, 0, 60)
	for range 55 {
		src = append(src, "a")
	}
	src = append(src, "b", "a", "c")
	r := &fts.Result{Hits: hits(src...), ModeUsed: "or"}
	got := FindReport(t.TempDir(), "q", r, 50)
	for _, s := range []string{"[b]", "[c]"} {
		if !strings.Contains(got, s) {
			t.Errorf("FindReport(-n 50) hides source %s:\n%s", s, got)
		}
	}
	if n := strings.Count(got, "  [a]"); n != 48 {
		t.Errorf("FindReport(-n 50) shows %d hits of a, want 48 (50 slots, 2 reserved)", n)
	}
	if want := "  ... and 8 more files\n"; !strings.HasSuffix(got, want) {
		t.Errorf("FindReport(-n 50) tail = %q, want %q", got[len(got)-40:], want)
	}
	// Without an explicit -n the ten-file cap and ranked order stay as they were.
	if got := FindCappedReport(t.TempDir(), "q", r, 50); strings.Contains(got, "[b]") {
		t.Errorf("FindReport(default) changed: shows b\n%s", got)
	}
}

// The raised pick still honours perSource and keeps rank order.
func TestFindReportRaisedPerSource(t *testing.T) {
	r := &fts.Result{Hits: hits("a", "a", "a", "b", "a", "c"), ModeUsed: "or"}
	h := r.Hits
	got := everySourceShown(r.Hits, 2, 20)
	want := []*fts.Hit{h[0], h[1], h[3], h[5]}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("everySourceShown = %v, want %v", got, want)
	}
	if got := everySourceShown(r.Hits, 0, 20); len(got) != 0 {
		t.Errorf("everySourceShown(perSource 0) = %v, want none", got)
	}
}
