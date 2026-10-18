package corpusnav

import (
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// corpusnav/search.py: find (ranked BM25, blocksearch) and grep (regex), with honest misses.

// searchOutcome is search.search_outcome: a miss names what was searched; unreadable
// files fail the run.
func (c *corpus) searchOutcome(found bool, scope string, nFiles int, unreadable []string) int {
	c.reportUnreadable(unreadable)
	if !found {
		fmt.Fprintln(c.out, blocksearch.MissLine(scope, nFiles, len(unreadable)))
	}
	switch {
	case len(unreadable) > 0:
		return 2
	case found:
		return 0
	}
	return 1
}

// cmdFind is search.cmd_find, ranked and rendered by blocksearch.
func (c *corpus) cmdFind(query, scope string, limit int, requireAll bool) int {
	r := c.search().Find(blocksearch.Query{Text: query, Scope: scope, Limit: limit, RequireAll: requireAll})
	if r.Note != "" {
		fmt.Fprintf(c.out, "(%s)\n", r.Note)
	}
	io.WriteString(c.err, r.CacheNote)
	io.WriteString(c.out, r.Rows)
	c.reportUnreadable(r.Unreadable)
	if r.Miss != "" {
		fmt.Fprintln(c.out, r.Miss)
	}
	return r.Exit()
}

func (c *corpus) cmdGrep(regex, scope string, limit int) int {
	catalog := c.loadCatalog()
	re, err := pyre.Compile(regex, pyre.IGNORECASE)
	if err != nil {
		fmt.Fprintf(c.err, "Invalid regex: %v\n", err)
		return 2
	}
	targets := c.targets(scope)
	var groups []blocksearch.Group
	var unreadable []string
	for _, t := range targets {
		text, ok := readText(t.path)
		if !ok {
			unreadable = append(unreadable, t.path)
			continue
		}
		g := blocksearch.Group{Display: t.display, Path: t.path}
		for n, ln := range pycompat.SplitLines(text, false) {
			if re.MatchString(ln) {
				g.Hits = append(g.Hits, blocksearch.Line{N: n + 1, Text: ln})
			}
		}
		if len(g.Hits) > 0 {
			groups = append(groups, g)
		}
	}
	i := 0
	io.WriteString(c.out, blocksearch.Render(func() (blocksearch.Group, bool) {
		if i >= len(groups) {
			return blocksearch.Group{}, false
		}
		i++
		return groups[i-1], true
	}, limit, catalog))
	return c.searchOutcome(len(groups) > 0, scope, len(targets), unreadable)
}
