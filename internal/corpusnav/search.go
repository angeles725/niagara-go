package corpusnav

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpusnav/search.py: find (ranked BM25) and grep (regex), with honest misses.

type lineHit struct {
	n    int
	text string
}

type group struct {
	display, path string
	hits          []lineHit
}

// resultHeader is search.result_header: "B<N> · <title>" for main blocks, else
// "<id or relpath> · <H1>".
func resultHeader(display, path string, catalog map[string]string) string {
	if label := mainLabelOf(display); label != "" {
		return display + " · " + titleFor(label, catalog, path)
	}
	return strings.TrimRight(display+" · "+readH1(path), " ·")
}

// printHits is search.print_hits over groups pulled one at a time from next (Python
// iterates a lazy generator); limit caps the TOTAL lines printed.
func (c *corpus) printHits(next func() (group, bool), limit int, catalog map[string]string) {
	shown := 0
	for i := 0; ; i++ {
		g, ok := next()
		if !ok || shown >= limit {
			return
		}
		if i > 0 {
			fmt.Fprintln(c.out)
		}
		fmt.Fprintln(c.out, resultHeader(g.display, g.path, catalog))
		for _, h := range g.hits {
			if shown >= limit {
				fmt.Fprintf(c.out, "    ... (limit %d reached; use --limit to see more)\n", limit)
				break
			}
			fmt.Fprintf(c.out, "  %s: %s\n", pycompat.RJust(fmt.Sprint(h.n), 6, ' '), pycompat.Strip(h.text))
			shown++
		}
	}
}

// searchOutcome is search.search_outcome: a miss names what was searched; unreadable
// files fail the run.
func (c *corpus) searchOutcome(found bool, scope string, nFiles int, unreadable []string) int {
	c.reportUnreadable(unreadable)
	if !found {
		counts := []honest.Count{{Label: "files", N: nFiles}}
		if len(unreadable) > 0 {
			counts = append(counts, honest.Count{Label: "unreadable", N: len(unreadable)})
		}
		fmt.Fprintln(c.out, honest.Miss("matches", scope, counts...))
	}
	switch {
	case len(unreadable) > 0:
		return 2
	case found:
		return 0
	}
	return 1
}

// cmdFind is search.cmd_find: files ranked by BM25 (title 8x); a line is shown when its
// folded text contains a query term. In OR mode a Spanish question also searches the
// English glossary terms at half weight and says so first.
func (c *corpus) cmdFind(query, scope string, limit int, requireAll bool) int {
	catalog := c.loadCatalog()
	var factors []bm25.Factor
	if !requireAll {
		factors = glossary.Default().WeightedTerms(query)
	}
	if len(factors) > 0 {
		fmt.Fprintf(c.out, "(%s)\n", glossary.Default().Note(query))
	}
	found := c.ranked(scope, query, requireAll, factors)
	qTerms := textfold.Terms(query)
	for _, f := range factors {
		qTerms = append(qTerms, f.Term)
	}
	unreadable := append([]string(nil), found.unreadable...)
	i := 0
	next := func() (group, bool) {
		for ; i < len(found.results); i++ {
			d := found.results[i].doc
			text, ok := d.textOf()
			if !ok { // gone or unreadable since the index was checked: an error, not a skip
				unreadable = append(unreadable, d.Path)
				continue
			}
			g := group{display: d.Display, path: d.Path}
			for n, ln := range pycompat.SplitLines(text, false) {
				folded := fold(ln)
				for _, q := range qTerms {
					if strings.Contains(folded, q) {
						g.hits = append(g.hits, lineHit{n + 1, ln})
						break
					}
				}
			}
			i++
			return g, true
		}
		return group{}, false
	}
	c.printHits(next, limit, catalog)
	return c.searchOutcome(len(found.results) > 0, scope, found.nFiles, unreadable)
}

func (c *corpus) cmdGrep(regex, scope string, limit int) int {
	catalog := c.loadCatalog()
	re, err := pyre.Compile(regex, pyre.IGNORECASE)
	if err != nil {
		fmt.Fprintf(c.err, "Invalid regex: %v\n", err)
		return 2
	}
	targets := c.targets(scope)
	var groups []group
	var unreadable []string
	for _, t := range targets {
		text, ok := readText(t.path)
		if !ok {
			unreadable = append(unreadable, t.path)
			continue
		}
		g := group{display: t.display, path: t.path}
		for n, ln := range pycompat.SplitLines(text, false) {
			if re.MatchString(ln) {
				g.hits = append(g.hits, lineHit{n + 1, ln})
			}
		}
		if len(g.hits) > 0 {
			groups = append(groups, g)
		}
	}
	i := 0
	c.printHits(func() (group, bool) {
		if i >= len(groups) {
			return group{}, false
		}
		i++
		return groups[i-1], true
	}, limit, catalog)
	return c.searchOutcome(len(groups) > 0, scope, len(targets), unreadable)
}
