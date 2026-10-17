package corpusnav

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/blockfiles"
)

// The connections, graph, path and lint-refs commands of corpusnav/graph.py.

// titled is graph._titled: "B<N> · <title>", "(not in corpus)" for a dangling label.
func (c *corpus) titled(label string, catalog map[string]string, byFile map[string]string) string {
	p, ok := byFile[label]
	if !ok {
		return displayID(label) + " · (not in corpus)"
	}
	return displayID(label) + " · " + titleFor(label, catalog, p)
}

func byFileOf(blocks []blockFile) map[string]string {
	m := map[string]string{}
	for _, b := range blocks {
		m[b.label] = b.path // dict(blocks): the last file of a label wins
	}
	return m
}

func labelSet(m map[string]string) map[string]bool {
	s := map[string]bool{}
	for k := range m {
		s[k] = true
	}
	return s
}

// checkUnreadable is graph._check_unreadable: report them; 2 if any (or a needed one).
func (c *corpus) checkUnreadable(g *graph, needed ...string) int {
	if len(g.unreadable) > 0 {
		ids := make([]string, len(g.unreadable))
		for i, x := range g.unreadable {
			ids[i] = displayID(x)
		}
		c.reportUnreadable(ids)
	}
	if len(needed) > 0 {
		for _, n := range needed {
			for _, u := range g.unreadable {
				if n == u {
					return 2
				}
			}
		}
		return 0
	}
	if len(g.unreadable) > 0 {
		return 2
	}
	return 0
}

// graphFor is graph._graph_for: resolve the typed ids and build the graph; code 1 when a
// block is not found, 2 when one of them is unreadable.
func (c *corpus) graphFor(raw []string, bare bool) ([]string, *graph, map[string]string, int) {
	blocks := c.blocks()
	byFile := byFileOf(blocks)
	labels := make([]string, len(raw))
	for i, x := range raw {
		labels[i] = resolveLabel(parseLabel(x), labelSet(byFile))
		if _, ok := byFile[labels[i]]; !ok {
			fmt.Fprintf(c.err, "Block %s not found.\n", displayID(labels[i]))
			return nil, nil, nil, 1
		}
	}
	g := buildGraph(blocks, bare)
	if c.checkUnreadable(g, labels...) != 0 {
		return nil, nil, nil, 2
	}
	return labels, g, byFile, 0
}

func (c *corpus) cmdConnections(n string, bare bool) int {
	labels, g, byFile, code := c.graphFor([]string{n}, bare)
	if code != 0 {
		return code
	}
	label, catalog := labels[0], c.loadCatalog()
	fmt.Fprintln(c.out, c.titled(label, catalog, byFile))
	for _, sec := range []struct {
		heading string
		ids     []string
	}{
		{"Forward links (this block -> others, citations anywhere in its text)", g.forward[label]},
		{"Reverse links (other blocks -> " + displayID(label) + ", whole-corpus scan)", g.reverse[label]},
	} {
		fmt.Fprintf(c.out, "\n## %s\n", sec.heading)
		for _, t := range sec.ids {
			fmt.Fprintf(c.out, "  %s\n", c.titled(t, catalog, byFile))
		}
		if len(sec.ids) == 0 {
			fmt.Fprintln(c.out, "  (none)")
		}
	}
	return 0
}

func (c *corpus) cmdGraph(asJSON bool, top int, bare bool) int {
	g := buildGraph(c.blocks(), bare)
	edges := 0
	for _, label := range g.nodes { // one node per label (buildGraph)
		edges += len(g.forward[label])
	}
	rc := c.checkUnreadable(g)
	if asJSON {
		ids := func(labels []string) []any {
			out := make([]any, len(labels))
			for i, x := range labels {
				out[i] = displayID(x)
			}
			return out
		}
		fwd, rev := obj{}, obj{}
		for _, k := range g.nodes {
			fwd = append(fwd, kv{displayID(k), ids(g.forward[k])})
		}
		for _, k := range g.reverseIDs {
			rev = append(rev, kv{displayID(k), ids(g.reverse[k])})
		}
		data := obj{{"nodes", ids(g.nodes)}, {"forward", fwd}, {"reverse", rev}, {"dangling", ids(g.dangling)},
			{"unreadable", ids(g.unreadable)}, {"stats", obj{{"nodes", len(g.nodes)}, {"edges", edges},
				{"dangling", len(g.dangling)}, {"unreadable", len(g.unreadable)}}}}
		fmt.Fprintln(c.out, jsonDumps(data))
		return rc
	}
	catalog, byFile := c.loadCatalog(), byFileOf(c.blocks())
	isolated := 0
	for _, n := range g.nodes {
		if len(g.forward[n]) == 0 && len(g.reverse[n]) == 0 {
			isolated++
		}
	}
	fmt.Fprintf(c.out, "Citation graph\n  nodes   : %d\n  edges   : %d\n", len(g.nodes), edges)
	fmt.Fprintf(c.out, "  dangling: %d cited block(s) with no file\n", len(g.dangling))
	fmt.Fprintf(c.out, "  isolated: %d block(s) citing and cited by nothing\n", isolated)
	fmt.Fprintln(c.out, "\nMost cited")
	order := append([]string(nil), g.reverseIDs...) // label order, then a stable sort by count
	sortStable(order, func(a, b string) bool { return len(g.reverse[a]) > len(g.reverse[b]) })
	for _, label := range order[:min(max(top, 0), len(order))] {
		fmt.Fprintf(c.out, "  %s  %s\n", padLeft(fmt.Sprint(len(g.reverse[label])), 5), c.titled(label, catalog, byFile))
	}
	return rc
}

func (c *corpus) cmdPath(a, b string, directed, bare bool) int {
	labels, g, byFile, code := c.graphFor([]string{a, b}, bare)
	if code != 0 {
		return code
	}
	catalog := c.loadCatalog()
	chain := shortestPath(g, labels[0], labels[1], directed)
	if chain == nil {
		mode := ""
		if directed {
			mode = "forward-only "
		}
		fmt.Fprintf(c.out, "No %spath between %s and %s.\n", mode, displayID(labels[0]), displayID(labels[1]))
		return 1
	}
	fmt.Fprintln(c.out, c.titled(chain[0], catalog, byFile))
	for i := 1; i < len(chain); i++ {
		arrow := "<-"
		for _, t := range g.forward[chain[i-1]] {
			if t == chain[i] {
				arrow = "->"
			}
		}
		fmt.Fprintf(c.out, "  %s %s\n", arrow, c.titled(chain[i], catalog, byFile))
	}
	fmt.Fprintf(c.out, "\n%d hop(s); '->' cites the next block, '<-' is cited by it.\n", len(chain)-1)
	return 0
}

func (c *corpus) cmdLintRefs(files []string, warn bool, limit int) int {
	limit = max(limit, 0) // positive_int on the command line; a caller's 0 or less prints none
	var targets []blockFile
	if len(files) > 0 {
		for _, f := range files {
			p := pyPath(f)
			label := blockfiles.MainLabel(baseName(p))
			if label == "" {
				label = f
			}
			targets = append(targets, blockFile{label: label, path: p})
		}
	} else {
		targets = c.blocks()
	}
	found, unreadable := lintRefs(targets)
	for _, f := range found[:min(limit, len(found))] {
		where := f.path
		if blockfiles.MainLabel(baseName(f.path)) != "" {
			where = displayID(f.block)
		}
		fmt.Fprintf(c.out, "%s:%d: %s %s  -> %s\n", where, f.line, padRight(f.form, 14), f.text, f.canonical)
	}
	if len(found) > limit {
		fmt.Fprintf(c.out, "... (%d more; use --limit to see more)\n", len(found)-limit)
	}
	counts, blocks := map[string]int{}, map[string]bool{}
	var forms []string
	for _, f := range found {
		if counts[f.form] == 0 {
			forms = append(forms, f.form)
		}
		counts[f.form]++
		blocks[f.block] = true
	}
	fmt.Fprintf(c.out, "\n%d non-canonical citation(s) in %d file(s) (canonical form: [B<N>] or [B<N> §x]; "+
		"report-only, nothing rewritten)\n", len(found), len(blocks))
	sortStable(forms, func(a, b string) bool {
		if counts[a] != counts[b] {
			return counts[a] > counts[b]
		}
		return a < b
	})
	for _, form := range forms {
		fmt.Fprintf(c.out, "  %s %s\n", padRight(form, 14), padLeft(fmt.Sprint(counts[form]), 6))
	}
	if len(unreadable) > 0 {
		c.reportUnreadable(unreadable)
		return 2 // even with --warn: an unread file is not a clean file
	}
	if len(found) > 0 && !warn {
		return 1
	}
	return 0
}

// padRight / padLeft are f"{s:<n}" / f"{s:>n}" (code points).
func padRight(s string, n int) string {
	if c := len([]rune(s)); c < n {
		return s + strings.Repeat(" ", n-c)
	}
	return s
}

func padLeft(s string, n int) string {
	if c := len([]rune(s)); c < n {
		return strings.Repeat(" ", n-c) + s
	}
	return s
}
