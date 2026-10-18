package searchdoor

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// findMaxResults is niagara_help find_search.cmd_find's max_results: ten files in all.
const findMaxResults = 10

// docsFind is `niagara_help [--niagara-version V] find <query> -n N [--and|--or]`
// (docsJob) as nh answers it, on the data root niagara_help.py would resolve:
// $NIAGARA_HELP_DIR, else the parent of its tools/ folder, then the requested (or
// default) version. A version that cannot be resolved prints nh's ERROR lines
// (exit 2); a missing or stale full-text index is rebuilt first, atomically and
// with nh's notices on stderr (help/scope.OpenFulltext, decision D38); a search
// that cannot run is nh's "ERROR: ..." line (exit 2); a miss is nh's honest-zero
// block (scope.Miss, exit 1).
func docsFind(argv []string) answer {
	args, version := argv[1:], ""
	if len(args) >= 2 && args[0] == "--niagara-version" {
		version, args = args[1], args[2:]
	}
	v, flag, ok := shape(args, []string{"find", "*", "-n", "#"}, "--and", "--or")
	if !ok {
		return noAnswer
	}
	repo := os.Getenv("NIAGARA_HELP_DIR")
	if repo == "" {
		repo = toolDir(argv[0], false)
	}
	if repo == "" {
		return noAnswer
	}
	id, root, err := scope.Resolve(repo, version, os.Getenv)
	if err != nil { // nh prints these on stdout
		return exact(2, strings.Join(scope.VersionLines(err), "\n")+"\n", "")
	}
	mode := "and"
	if flag == "--or" {
		mode = "or"
	}
	var stderr strings.Builder
	cannotSearch := func(err error) answer { // nh's run: the ERROR line on stderr, exit 2
		return exact(2, "", stderr.String()+scope.ErrorLine(err)+"\n")
	}
	query := v[0]
	r, err := fts.SearchUsing(scope.OpenFulltext(&stderr), root, id, query, nil, mode)
	if err != nil {
		return cannotSearch(err)
	}
	if len(r.Hits) == 0 {
		lines, missErr := scope.Miss(root, "find", query, "fulltext", nil)
		if missErr != nil {
			return cannotSearch(missErr)
		}
		return exact(1, strings.Join(lines, "\n")+"\n", stderr.String())
	}
	perSource, _ := strconv.Atoi(v[1])
	return exact(0, renderFind(root, query, r, perSource), stderr.String())
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
