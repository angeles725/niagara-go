package fts

// Ranked search (fulltext.py search): strict AND, relaxed AND, then a ranked
// OR fallback, with the BM25 x coverage² x source factor document score and
// the per-file aggregation over pages.

import (
	"regexp"
	"slices"
	"sort"

	"github.com/angeles725/niagara-go/internal/bm25"
)

const (
	// TitleWeight is fulltext.TITLE_WEIGHT, the bm25() weight of the title column.
	TitleWeight = 3.0
	// MinAndHits is fulltext.MIN_AND_HITS: AND mode is kept only when it
	// finds this many documentation files.
	MinAndHits      = 10
	changelogFactor = 0.25
)

// pageDecay holds fulltext.PAGE_DECAY ** i (0.85 ** i) for i < 8, the exact
// doubles CPython's float pow returns, so no Go pow rounding can differ.
var pageDecay = [8]float64{0x1p+0, 0x1.b333333333333p-1, 0x1.71eb851eb851ep-1,
	0x1.3a6e978d4fdf3p-1, 0x1.0b4467381d7dbp-1, 0x1.c65aaf78feef5p-2,
	0x1.82337b8d3f183p-2, 0x1.48455c380f3afp-2}

// sourceFactor is fulltext.SOURCE_FACTOR: code ranks below an equal doc page.
var sourceFactor = map[string]float64{"source": 0.5, "devguide-rc": 0.5}

// changelog is fulltext._CHANGELOG: release notes and change logs.
var changelog = regexp.MustCompile(`(?i)(^|/)(changes\d*|releasenotes)[^/]*$`)

// isChangelog is _CHANGELOG.search(path). Python's \d is any Unicode digit
// and re.I folds non-ASCII letters; neither changes a match of these ASCII
// names except a non-ASCII digit after "changes", which Go's \d misses.
func isChangelog(path string) bool { return changelog.MatchString(path) }

// UsageError is scope.UsageError: an invalid query (exit 2).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// Hit is fulltext.Hit: one matching file, its best document's title, line
// and page, its aggregated score and the groups that document matched.
type Hit struct {
	Source, Path, Title string
	Score               float64
	Line, Page          int
	Coverage            int
}

// Key is Hit.key: "<source>/<path>".
func (h *Hit) Key() string { return h.Source + "/" + h.Path }

// Result is fulltext.Result: the hits and how they were found.
type Result struct {
	Groups   []Group
	Hits     []*Hit
	ModeUsed string // "and" or "or"
	Fallback bool   // AND found too few files and fell back to a ranked OR
	AndHits  int    // files with every word, on a fallback
}

type doc struct {
	score      float64
	cov        int
	title      string
	line, page int
}

// rank is fulltext._rank: one Hit per file, best first. A document scores
// -bm25 x (coverage / groups)² x source factor (x 0.25 for a change log); a
// file scores its best document plus PAGE_DECAY-weighted further ones (eight
// at most), summed like Python's sum().
func rank(rows []row, coverageOf func(id int64) int, nGroups int) []*Hit {
	type file struct{ source, path string }
	var order []file
	byFile := map[file][]doc{}
	for _, r := range rows {
		cov := coverageOf(r.id)
		ratio := float64(cov) / float64(nGroups)
		f, ok := sourceFactor[r.source]
		if !ok {
			f = 1.0
		}
		// (cov / n) ** 2 is glibc pow(x, 2.0); it equals x*x for every
		// cov <= n <= 40 (first difference: 33/41), so up to 40 query words.
		score := -r.bm25 * (ratio * ratio) * f
		if isChangelog(r.path) {
			score *= changelogFactor
		}
		k := file{r.source, r.path}
		if _, seen := byFile[k]; !seen {
			order = append(order, k)
		}
		byFile[k] = append(byFile[k], doc{score: score, cov: cov, title: r.title, line: r.line, page: r.page})
	}
	hits := make([]*Hit, 0, len(order))
	for _, k := range order {
		docs := byFile[k]
		sort.SliceStable(docs, func(i, j int) bool { return -docs[i].score < -docs[j].score })
		n := min(len(docs), len(pageDecay))
		terms := make([]float64, n)
		for i := 0; i < n; i++ {
			terms[i] = docs[i].score * pageDecay[i]
		}
		best := docs[0]
		hits = append(hits, &Hit{Source: k.source, Path: k.path, Title: best.title,
			Score: bm25.PythonSum(terms), Line: best.line, Page: best.page, Coverage: best.cov})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Score != b.Score {
			return -a.Score < -b.Score
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Path < b.Path
	})
	return hits
}

// rankedOr is fulltext._ranked_or: every document matching any group,
// coverage counted per document.
func (x *Index) rankedOr(groups []Group, srcs []string) ([]*Hit, error) {
	sets, err := x.groupIDs(groups, srcs)
	if err != nil {
		return nil, err
	}
	rows, err := x.rows(groups, "or", srcs, TitleWeight)
	if err != nil {
		return nil, err
	}
	return rank(rows, func(id int64) int {
		n := 0
		for _, s := range sets {
			if s[id] {
				n++
			}
		}
		return n
	}, len(groups)), nil
}

// orFallback is fulltext._or_fallback: the ranked OR of the strict groups,
// then the files only the relaxed (camelCase part) groups match.
func (x *Index) orFallback(strict, relaxed []Group, srcs []string) ([]*Hit, error) {
	hits, err := x.rankedOr(strict, srcs)
	if err != nil || groupsEqual(strict, relaxed) {
		return hits, err
	}
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.Key()] = true
	}
	more, err := x.rankedOr(relaxed, srcs)
	if err != nil {
		return nil, err
	}
	for _, h := range more {
		if !seen[h.Key()] {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

func groupsEqual(a, b []Group) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].equal(b[i]) {
			return false
		}
	}
	return true
}

// codeSources are fulltext.CODE_SOURCES.
func isCode(source string) bool { _, ok := sourceFactor[source]; return ok }

// enough is fulltext._enough: the hits hold MIN_AND_HITS files that are
// neither change logs nor code (code counts when only code is searched).
func enough(hits []*Hit, srcs []string) bool {
	codeOnly := len(srcs) > 0
	for _, s := range srcs {
		codeOnly = codeOnly && isCode(s)
	}
	n := 0
	for _, h := range hits {
		if !isChangelog(h.Path) && (codeOnly || !isCode(h.Source)) {
			n++
		}
	}
	return n >= MinAndHits
}

// Search is fulltext.search: rank the files of srcs (labels; nil means every
// source) for query. mode "and" tries strict AND, then relaxed AND when that
// found nothing, and keeps the AND result when it has MIN_AND_HITS
// documentation files (or the query is one word); otherwise, and in mode
// "or", it is the ranked OR fallback. The index is opened (Open) only when
// the query has a word and some requested source exists.
func Search(baseDir, versionID, query string, srcs []string, mode string) (*Result, error) {
	return SearchUsing(func(baseDir, versionID, _ string) (*Index, error) { return Open(baseDir, versionID) },
		baseDir, versionID, query, srcs, mode)
}

// SearchUsing is Search with the index opened by open, at the point where
// fulltext.search calls open_index (so a rebuilding opener builds nothing
// for a query without words or without an available source).
func SearchUsing(open Opener, baseDir, versionID, query string, srcs []string, mode string) (*Result, error) {
	strict := ParseQuery(query, false)
	if len(strict) == 0 {
		return nil, &UsageError{"query '" + Echo(query) + "' has no searchable word"}
	}
	var available []string
	for _, d := range SourceDirs(baseDir) {
		available = append(available, d.Label)
	}
	if len(srcs) == 0 {
		srcs = available
	}
	var use []string
	for _, s := range srcs {
		if slices.Contains(available, s) {
			use = append(use, s)
		}
	}
	if len(use) == 0 {
		return &Result{Groups: strict, ModeUsed: mode}, nil
	}
	relaxed := ParseQuery(query, true)
	x, err := open(baseDir, versionID, query)
	if err != nil {
		return nil, err
	}
	defer x.Close()
	if mode != "and" {
		hits, err := x.orFallback(strict, relaxed, use)
		return &Result{Groups: relaxed, Hits: hits, ModeUsed: "or"}, err
	}
	n := len(strict)
	all := func(int64) int { return n }
	rows, err := x.rows(strict, "and", use, TitleWeight)
	if err != nil {
		return nil, err
	}
	hits, groups := rank(rows, all, n), strict
	if len(hits) == 0 && !groupsEqual(relaxed, strict) {
		if rows, err = x.rows(relaxed, "and", use, TitleWeight); err != nil {
			return nil, err
		}
		hits, groups = rank(rows, all, n), relaxed
	}
	if n == 1 || enough(hits, use) {
		return &Result{Groups: groups, Hits: hits, ModeUsed: "and"}, nil
	}
	or, err := x.orFallback(strict, relaxed, use)
	return &Result{Groups: relaxed, Hits: or, ModeUsed: "or", Fallback: true, AndHits: len(hits)}, err
}
