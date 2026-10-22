package scope

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// findMaxResults is find_search.cmd_find's max_results: ten files in all.
const findMaxResults = 10

// FindReport is nh find with an explicit -n perSource, as nh and the
// in-process searchdoor (which always passes -n) print it: findReport raised.
func FindReport(root, query string, r *fts.Result, perSource int) string {
	return findReport(root, query, r, perSource, true)
}

// FindCappedReport is find_search.cmd_find's own cap: at most perSource hits
// per source and ten in all. nh prints it when -n is absent (perSource 5).
func FindCappedReport(root, query string, r *fts.Result, perSource int) string {
	return findReport(root, query, r, perSource, false)
}

// findReport is find_search.cmd_find's output for a result with hits: one
// globally ranked list, at most perSource hits per source and ten in all,
// under the per-source totals and the notes of borrowed sources.
//
// raised is an explicit -n: the total becomes max(ten, perSource) and every
// source with hits keeps its best hit, so a source the totals line counts is
// never left without a row (the others fill the remaining slots in rank
// order). Without it the output is cmd_find's.
func findReport(root, query string, r *fts.Result, perSource int, raised bool) string {
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
	if raised {
		shown = everySourceShown(r.Hits, perSource, max(findMaxResults, perSource))
	} else {
		used := map[string]int{}
		for _, h := range r.Hits {
			if used[h.Source] < perSource {
				shown = append(shown, h)
				used[h.Source]++
			}
			if len(shown) >= findMaxResults {
				break
			}
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
	// find_search.shared_notes: each (sorted) label borrowed from another version.
	sort.Strings(labels)
	for _, l := range labels {
		if note, ok := fts.Shared[l]; ok {
			if _, owner := fts.SharedRoot(root, l); owner != "" {
				totals += "\n  (" + l + ": " + fmt.Sprintf(note, owner) + ")"
			}
		}
	}
	return RankedReport(root, query, r, shown, "files", totals, func(h *fts.Hit) string { return h.Source })
}

// RankedReport is find_search.print_ranked: the header (how the hits were
// found), an optional totals line, then per shown hit its tag and title,
// best line and path.
func RankedReport(root, query string, r *fts.Result, shown []*fts.Hit, label, totals string,
	tag func(*fts.Hit) string) string {
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
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s matching '%s' (%s):\n", len(r.Hits), label, query, how)
	if totals != "" {
		b.WriteString(totals + "\n")
	}
	b.WriteString("\n")
	dirs := fts.SourceDirs(root)
	for _, h := range shown {
		fmt.Fprintln(&b, fts.FormatHit(dirs, h, r.Groups, tag(h)))
	}
	if len(r.Hits) > len(shown) {
		fmt.Fprintf(&b, "  ... and %d more files\n", len(r.Hits)-len(shown))
	}
	return b.String()
}

// everySourceShown picks, in rank order, at most perSource hits per source
// and total in all, reserving the best hit of every source first (when
// perSource allows one), so no source with hits is invisible. More sources
// than total each still get their one row.
func everySourceShown(ranked []*fts.Hit, perSource, total int) []*fts.Hit {
	best := map[*fts.Hit]bool{}
	seen := map[string]bool{}
	if perSource > 0 {
		for _, h := range ranked {
			if !seen[h.Source] {
				seen[h.Source] = true
				best[h] = true
			}
		}
	}
	reserved := len(best)
	var shown []*fts.Hit
	used := map[string]int{}
	for _, h := range ranked {
		switch {
		case best[h]:
			reserved--
		case used[h.Source] < perSource && len(shown)+reserved < total:
		default:
			continue
		}
		shown = append(shown, h)
		used[h.Source]++
	}
	return shown
}
