package blocksearch

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpusnav/search.py cmd_find as data: the ranked groups, the honest miss and the
// exit status, plus the exact lines corpus-nav find prints for them.

// Line is one hit line of a file: its 1-based number and raw text.
type Line struct {
	N    int
	Text string
}

// Group is one result file: its display id, path and the lines that matched.
type Group struct {
	Display, Path string
	Hits          []Line
}

// Query is one find: the text, the scope (blocks, docs, retros, all), the cap on
// printed hit lines and AND mode.
type Query struct {
	Text, Scope string
	Limit       int
	RequireAll  bool
}

// Result is a finished find. Stdout is everything corpus-nav find prints there,
// in order: the glossary note, the result groups, the miss line.
type Result struct {
	Note       string   // the glossary note shown first, without its parentheses, or ""
	Rows       string   // the rendered groups (Render)
	Miss       string   // the honest miss line when nothing matched, else ""
	CacheNote  string   // the stderr line of a cache rebuild, or ""
	Unreadable []string // files that could not be read (each one stderr line, exit 2)
	Found      bool
}

// Exit is search_outcome's status: 2 when a file was unreadable, else 0 found, 1 not.
func (r *Result) Exit() int {
	switch {
	case len(r.Unreadable) > 0:
		return 2
	case r.Found:
		return 0
	}
	return 1
}

// Stdout is the exact standard output of corpus-nav find.
func (r *Result) Stdout() string {
	var b strings.Builder
	if r.Note != "" {
		fmt.Fprintf(&b, "(%s)\n", r.Note)
	}
	b.WriteString(r.Rows)
	if r.Miss != "" {
		b.WriteString(r.Miss + "\n")
	}
	return b.String()
}

// Stderr is the exact standard error of corpus-nav find: the cache line, then one
// line per unreadable file.
func (r *Result) Stderr() string {
	var b strings.Builder
	b.WriteString(r.CacheNote)
	for _, p := range r.Unreadable {
		b.WriteString(UnreadableLine(p) + "\n")
	}
	return b.String()
}

// UnreadableLine is corpus.report_unreadable's line for one file.
func UnreadableLine(path string) string {
	return "corpus-nav: cannot read " + path + " (missing, unreadable or not UTF-8)"
}

// Header is search.result_header: "B<N> · <title>" for main blocks, else
// "<id or relpath> · <H1>".
func Header(display, path string, catalog map[string]string) string {
	if label := MainLabel(display); label != "" {
		return display + " · " + TitleFor(label, catalog, path)
	}
	return strings.TrimRight(display+" · "+ReadH1(path), " ·")
}

// Render is search.print_hits over groups pulled one at a time from next (Python
// iterates a lazy generator: one group past the limit is still pulled); limit caps
// the TOTAL hit lines.
func Render(next func() (Group, bool), limit int, catalog map[string]string) string {
	var b strings.Builder
	shown := 0
	for i := 0; ; i++ {
		g, ok := next()
		if !ok || shown >= limit {
			return b.String()
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(Header(g.Display, g.Path, catalog) + "\n")
		for _, h := range g.Hits {
			if shown >= limit {
				fmt.Fprintf(&b, "    ... (limit %d reached; use --limit to see more)\n", limit)
				break
			}
			fmt.Fprintf(&b, "  %s: %s\n", pycompat.RJust(fmt.Sprint(h.N), 6, ' '), pycompat.Strip(h.Text))
			shown++
		}
	}
}

// MissLine is search_outcome's miss: what was searched, with the file counts.
func MissLine(scope string, nFiles, nUnreadable int) string {
	counts := []honest.Count{{Label: "files", N: nFiles}}
	if nUnreadable > 0 {
		counts = append(counts, honest.Count{Label: "unreadable", N: nUnreadable})
	}
	return honest.Miss("matches", scope, counts...)
}

// Find is search.cmd_find: files ranked by BM25 (title 8x); a line is shown when its
// folded text contains a query term. In OR mode a Spanish question also searches the
// English glossary terms at half weight and says so first.
func (c *Corpus) Find(q Query) *Result {
	catalog := c.LoadCatalog()
	var factors []bm25.Factor
	if !q.RequireAll {
		factors = glossary.Default().WeightedTerms(q.Text)
	}
	r := &Result{}
	if len(factors) > 0 {
		r.Note = glossary.Default().Note(q.Text)
	}
	found := c.Ranked(q.Scope, q.Text, q.RequireAll, factors)
	r.CacheNote = found.Note
	qTerms := textfold.Terms(q.Text)
	for _, f := range factors {
		qTerms = append(qTerms, f.Term)
	}
	r.Unreadable = append([]string(nil), found.Unreadable...)
	i := 0
	next := func() (Group, bool) {
		for ; i < len(found.Results); i++ {
			d := found.Results[i].Doc
			text, ok := d.Text()
			if !ok { // gone or unreadable since the index was checked: an error, not a skip
				r.Unreadable = append(r.Unreadable, d.Path)
				continue
			}
			g := Group{Display: d.Display, Path: d.Path}
			for n, ln := range pycompat.SplitLines(text, false) {
				folded := fold(ln)
				for _, t := range qTerms {
					if strings.Contains(folded, t) {
						g.Hits = append(g.Hits, Line{n + 1, ln})
						break
					}
				}
			}
			i++
			return g, true
		}
		return Group{}, false
	}
	r.Rows = Render(next, q.Limit, catalog)
	r.Found = len(found.Results) > 0
	if !r.Found {
		r.Miss = MissLine(q.Scope, found.NFiles, len(r.Unreadable))
	}
	return r
}
