package help

// The ranked full-text commands (find_search.py, guide_search.py
// guide-search / devguide-search): fts ranks, these print. The index is
// fulltext.sqlite, rebuilt first when it is missing or stale like Python's
// open_index (scope.OpenFulltext, decision D38).

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/pycompat"
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

// search runs fts.Search on the selected data root, rebuilding a missing or
// stale index first (scope.OpenFulltext).
func (e *Env) search(query string, srcs []string, mode string) (*fts.Result, error) {
	return fts.SearchUsing(scope.OpenFulltext(e.Err), e.BaseDir, e.VersionID, query, srcs, mode)
}

// ftHead is Python's items[:n] for a slice length: a negative n counts from
// the end.
func ftHead(length, n int) int {
	if n < 0 {
		n += length
	}
	return max(0, min(n, length))
}

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
	fmt.Fprint(e.Out, scope.FindReport(e.BaseDir, p.Query, r, p.Max))
	return nil
}

// rankedSearch is guide_search._ranked_search: the full-text search
// restricted to srcs, a miss when the first source folder is absent.
func (e *Env) rankedSearch(p FulltextParams, srcs []string, command, where string,
	tag func(*fts.Hit) string) error {
	if !pycompat.IsDir(filepath.Join(e.BaseDir, srcs[0])) {
		return e.reportMiss(command, p.Query, where)
	}
	r, err := e.search(p.Query, srcs, p.mode())
	if err != nil {
		return err
	}
	if len(r.Hits) == 0 {
		return e.reportMiss(command, p.Query, where)
	}
	label, _, _ := strings.Cut(srcs[0], "-")
	shown := r.Hits[:ftHead(len(r.Hits), p.Max)]
	fmt.Fprint(e.Out, scope.RankedReport(e.BaseDir, p.Query, r, shown, label+" files", "", tag))
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
