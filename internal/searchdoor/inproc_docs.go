package searchdoor

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
)

// findMaxResults is niagara_help find_search.cmd_find's max_results: ten files in all.
const findMaxResults = 10

// docsFind is `niagara_help [--niagara-version V] find <query> -n N [--and|--or]`
// (docsJob) through fts, on the data root niagara_help.py would resolve:
// $NIAGARA_HELP_DIR, else the parent of its tools/ folder, then the requested (or
// default) version. Exec answers when the version cannot be resolved, when the
// full-text index is missing or stale (D21: Go never builds it, Python does) or
// when nothing matches (the honest-zero block is niagara-help's own printing).
func docsFind(argv []string) answer {
	args, version := argv[1:], ""
	if len(args) >= 2 && args[0] == "--niagara-version" {
		version, args = args[1], args[2:]
	}
	v, flag, ok := shape(args, []string{"find", "*", "-n", "#"}, "--and", "--or")
	if !ok || strings.HasPrefix(version, "-") {
		return viaExec
	}
	repo := os.Getenv("NIAGARA_HELP_DIR")
	if repo == "" {
		repo = toolDir(argv[0], false)
	}
	if repo == "" {
		return viaExec
	}
	id, root, err := index.Resolve(repo, version, os.Getenv)
	if err != nil || (id != "" && !index.IsExtracted(root)) {
		return viaExec
	}
	mode := "and"
	if flag == "--or" {
		mode = "or"
	}
	r, err := fts.Search(root, id, v[0], nil, mode)
	if err != nil || len(r.Hits) == 0 {
		return viaExec
	}
	perSource, _ := strconv.Atoi(v[1])
	return exact(0, renderFind(root, v[0], r, perSource), "")
}

// renderFind is find_search.cmd_find + print_ranked for a result with hits: the
// header (how the hits were found), the per-source totals with the notes of
// borrowed sources, then at most perSource hits per source and ten in all.
func renderFind(root, query string, r *fts.Result, perSource int) string {
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
		if used[h.Source] < perSource {
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
	sort.Strings(labels)
	for _, l := range labels {
		if note, ok := fts.Shared[l]; ok {
			if _, owner := fts.SharedRoot(root, l); owner != "" {
				totals += "\n  (" + l + ": " + fmt.Sprintf(note, owner) + ")"
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d files matching '%s' (%s):\n%s\n\n", len(r.Hits), query, foundHow(r), totals)
	dirs := fts.SourceDirs(root)
	for _, h := range shown {
		fmt.Fprintln(&b, fts.FormatHit(dirs, h, r.Groups, h.Source))
	}
	if len(r.Hits) > len(shown) {
		fmt.Fprintf(&b, "  ... and %d more files\n", len(r.Hits)-len(shown))
	}
	return b.String()
}

// foundHow is print_ranked's "(how)" of the header line.
func foundHow(r *fts.Result) string {
	if !r.Fallback {
		if r.ModeUsed == "and" {
			return "all terms"
		}
		return "any term, ranked by how many terms match"
	}
	every := "no file has every term"
	switch {
	case r.AndHits == 1:
		every = "only 1 file has every term"
	case r.AndHits > 1:
		every = fmt.Sprintf("only %d files have every term", r.AndHits)
	}
	return every + "; showing files with any term, ranked by how many terms match"
}
