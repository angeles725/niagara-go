package help

// The ranked full-text commands (find_search.py, guide_search.py
// guide-search / devguide-search): fts ranks, these print. The index is the
// Python-built fulltext.sqlite, read only (fts.Open).

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// FulltextParams are the arguments of find, guide-search and devguide-search.
type FulltextParams struct {
	Query string
	Max   int  // find: hits per source (-n); guide/devguide-search: hits shown (-n)
	Or    bool // --or: any term, ranked
}

func (p FulltextParams) mode() string {
	if p.Or {
		return "or"
	}
	return "and"
}

// search runs fts.Search on the selected data root.
func (e *Env) search(query string, srcs []string, mode string) (*fts.Result, error) {
	return fts.Search(e.BaseDir, e.VersionID, query, srcs, mode)
}

// ftHead is Python's items[:n] for a slice length: a negative n counts from
// the end.
func ftHead(length, n int) int {
	if n < 0 {
		n += length
	}
	return max(0, min(n, length))
}

// printRanked is find_search.print_ranked: the header (how the hits were
// found), an optional totals line, then per shown hit its tag and title,
// best line and path.
func (e *Env) printRanked(query string, r *fts.Result, shown []*fts.Hit, label, totals string,
	tag func(*fts.Hit) string) {
	how := "any term, ranked by how many terms match"
	if r.ModeUsed == "and" && !r.Fallback {
		how = "all terms"
	}
	if r.Fallback {
		every := "no file has every term"
		switch {
		case r.AndHits == 1:
			every = "only 1 file has every term"
		case r.AndHits > 1:
			every = fmt.Sprintf("only %d files have every term", r.AndHits)
		}
		how = every + "; showing files with any term, ranked by how many terms match"
	}
	fmt.Fprintf(e.Out, "%d %s matching '%s' (%s):\n", len(r.Hits), label, query, how)
	if totals != "" {
		fmt.Fprintln(e.Out, totals)
	}
	fmt.Fprintln(e.Out)
	dirs := fts.SourceDirs(e.BaseDir)
	for _, h := range shown {
		fmt.Fprintln(e.Out, fts.FormatHit(dirs, h, r.Groups, tag(h)))
	}
	if len(r.Hits) > len(shown) {
		fmt.Fprintf(e.Out, "  ... and %d more files\n", len(r.Hits)-len(shown))
	}
}

// findMaxResults is cmd_find's max_results: the list never exceeds ten files.
const findMaxResults = 10

// Find is find_search.cmd_find: one globally ranked list over every text
// source, at most Max hits per source and ten in all, with the per-source
// totals and the notes of borrowed sources.
func (e *Env) Find(p FulltextParams) error {
	r, err := e.search(p.Query, nil, p.mode())
	if err != nil {
		return err
	}
	if len(r.Hits) == 0 {
		return e.reportMiss("find", p.Query, "fulltext")
	}
	type count struct {
		source string
		n      int
	}
	var counts []count
	at := map[string]int{}
	for _, h := range r.Hits {
		if i, ok := at[h.Source]; ok {
			counts[i].n++
		} else {
			at[h.Source] = len(counts)
			counts = append(counts, count{h.Source, 1})
		}
	}
	var shown []*fts.Hit
	used := map[string]int{}
	for _, h := range r.Hits {
		if used[h.Source] < p.Max {
			shown = append(shown, h)
			used[h.Source]++
		}
		if len(shown) >= findMaxResults {
			break
		}
	}
	sort.SliceStable(counts, func(i, j int) bool { return counts[i].n > counts[j].n })
	parts := make([]string, len(counts))
	labels := make([]string, len(counts))
	for i, c := range counts {
		parts[i] = fmt.Sprintf("%s %d", c.source, c.n)
		labels[i] = c.source
	}
	totals := "by source: " + strings.Join(parts, ", ")
	for _, n := range e.sharedNotes(labels) {
		totals += "\n  (" + n + ")"
	}
	e.printRanked(p.Query, r, shown, "files", totals, func(h *fts.Hit) string { return h.Source })
	return nil
}

// sharedNotes is find_search.shared_notes: "<label>: <note>" for each label
// (sorted) this version borrows from another registered version.
func (e *Env) sharedNotes(labels []string) []string {
	sorted := append([]string(nil), labels...)
	sort.Strings(sorted)
	var out []string
	for _, l := range sorted {
		note, ok := fts.Shared[l]
		if !ok {
			continue
		}
		if _, owner := fts.SharedRoot(e.BaseDir, l); owner != "" {
			out = append(out, l+": "+fmt.Sprintf(note, owner))
		}
	}
	return out
}

// rankedSearch is guide_search._ranked_search: the full-text search
// restricted to srcs, a miss when the first source folder is absent.
func (e *Env) rankedSearch(p FulltextParams, srcs []string, command, scope string,
	tag func(*fts.Hit) string) error {
	if !isDir(filepath.Join(e.BaseDir, srcs[0])) {
		return e.reportMiss(command, p.Query, scope)
	}
	r, err := e.search(p.Query, srcs, p.mode())
	if err != nil {
		return err
	}
	if len(r.Hits) == 0 {
		return e.reportMiss(command, p.Query, scope)
	}
	label, _, _ := strings.Cut(srcs[0], "-")
	e.printRanked(p.Query, r, r.Hits[:ftHead(len(r.Hits), p.Max)], label+" files", "", tag)
	return nil
}

// GuideSearch is guide_search.cmd_guide_search over guides-clean/; the tag
// is the guide folder.
func (e *Env) GuideSearch(p FulltextParams) error {
	return e.rankedSearch(p, []string{"guides-clean"}, "guide-search", "guides", func(h *fts.Hit) string {
		if first, _, ok := strings.Cut(h.Path, "/"); ok {
			return first
		}
		return h.Source
	})
}

// DevguideSearch is guide_search.cmd_devguide_search over devguide-clean/
// and the developer examples.
func (e *Env) DevguideSearch(p FulltextParams) error {
	return e.rankedSearch(p, []string{"devguide-clean", "devguide-examples"}, "devguide-search", "devguide",
		func(h *fts.Hit) string {
			if h.Source == "devguide-examples" {
				return "example"
			}
			return "devguide"
		})
}
