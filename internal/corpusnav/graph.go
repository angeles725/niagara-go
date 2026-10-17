package corpusnav

import (
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// corpusnav/graph.py: the one block-reference extractor and the citation graph.

// refRe is _REF_RE: [Block N] / [Bloque N] / [Bloque #N], [BN] / [BN §x], bloqueN.
var refRe = pyre.MustCompile(`
    [\[Bb]
    (?:
        (?<=\[) [Bb][Ll][Oo](?:[Cc][Kk]|[Qq][Uu][Ee])\s+\#?(\d+)\]   # [Block 18] / [Bloque 75] / [Bloque #12]
      | (?<=\[) [Bb](\d+)(?:\]|\s+§)                                # [B722] / [B722 §3.1] (section anchor)
      | (?<!\w.)(?<=[Bb]) [Ll][Oo][Qq][Uu][Ee](\d+)\b                 # bloque106 (at a word start, like \b)
    )
    `, pyre.VERBOSE)

var (
	// bareRefRe is _BARE_REF_RE (--bare), codeRe is _CODE_RE (code spans and fences).
	bareRefRe = pyre.MustCompile(`(?<![\w/.\[-])B(\d{1,4})(?![\w-])`, 0)
	codeRe    = pyre.MustCompile("```.*?```|`[^`\\n]*`", pyre.DOTALL)
)

// parseLabel is graph.parse_label: "B720", "720", "b0720", "TI" -> a label.
func parseLabel(raw string) string {
	s := pycompat.Strip(raw)
	if (strings.HasPrefix(s, "B") || strings.HasPrefix(s, "b")) && len(s) > 1 {
		s = s[1:]
	}
	if isDigits(s) {
		return pyInt(s)
	}
	if strings.ToUpper(s) == "TI" {
		return "TI"
	}
	return s
}

// resolveLabel maps 1, 2 and 3 to the consolidated "1-3" when they have no file.
func resolveLabel(label string, labels map[string]bool) string {
	if (label == "1" || label == "2" || label == "3") && !labels[label] && labels["1-3"] {
		return "1-3"
	}
	return label
}

// refsInText is graph.refs_in_text: every block number cited (bare B<N> too with bare).
func refsInText(text string, bare bool) map[string]bool {
	nums := map[string]bool{}
	for _, m := range refRe.FindIter([]rune(text)) {
		for g := 1; g <= 3; g++ {
			if s := m.Group(g); s != "" {
				nums[pyInt(s)] = true
			}
		}
	}
	if bare {
		for _, n := range bareRefRe.FindAll(subSpans(text, codeRe, " ")) {
			nums[pyInt(n)] = true
		}
	}
	return nums
}

// subSpans is rx.sub(repl, text) for a fixed replacement.
func subSpans(text string, rx *pyre.Regexp, repl string) string {
	rs := []rune(text)
	var b strings.Builder
	last := 0
	for _, m := range rx.FindIter(rs) {
		b.WriteString(string(rs[last:m.Start()]))
		b.WriteString(repl)
		last = m.End()
	}
	b.WriteString(string(rs[last:]))
	return b.String()
}

// graph is build_graph's result: labels sorted, reverse and dangling included.
type graph struct {
	nodes      []string
	forward    map[string][]string
	reverse    map[string][]string
	reverseIDs []string // reverse keys in label order
	dangling   []string
	unreadable []string
}

// pyInt is str(int(digits)) for the Unicode decimal digits re's \d matched.
func pyInt(digits string) string {
	n, err := pycompat.ParseBigInt(digits)
	if err != nil {
		return "0"
	}
	return n.String()
}

func isDigits(s string) bool {
	for _, r := range s {
		if !pycompat.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

func setKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// buildGraph is graph.build_graph: block A -> B when A's text cites B; unreadable blocks
// are left out; citations of B1-B3 land on "1-3"; bare mentions only reach existing blocks.
func buildGraph(blocks []blockFile, bare bool) *graph {
	g := &graph{forward: map[string][]string{}, reverse: map[string][]string{}}
	texts := map[string]string{}
	for _, b := range blocks {
		if text, ok := readText(b.path); ok {
			texts[b.label] = text
		} else {
			g.unreadable = append(g.unreadable, b.label)
		}
	}
	known := map[string]bool{}
	for _, b := range blocks {
		if _, ok := texts[b.label]; ok {
			g.nodes = append(g.nodes, b.label)
			g.reverse[b.label] = []string{}
		}
		known[b.label] = true
	}
	for _, label := range g.nodes {
		cited := map[string]bool{}
		for n := range refsInText(texts[label], false) {
			cited[resolveLabel(n, known)] = true
		}
		if bare {
			for n := range refsInText(texts[label], true) {
				if t := resolveLabel(n, known); known[t] {
					cited[t] = true
				}
			}
		}
		delete(cited, label)
		targets := setKeys(cited)
		blockfiles.SortLabels(targets)
		g.forward[label] = targets
		for _, t := range targets {
			g.reverse[t] = append(g.reverse[t], label)
		}
	}
	for k := range g.reverse {
		g.reverseIDs = append(g.reverseIDs, k)
		if !known[k] {
			g.dangling = append(g.dangling, k)
		}
	}
	blockfiles.SortLabels(g.reverseIDs)
	blockfiles.SortLabels(g.dangling)
	return g
}

// shortestPath is graph.shortest_path: BFS over readable blocks, neighbours in label
// order, either direction unless directed.
func shortestPath(g *graph, src, dst string, directed bool) []string {
	_, inF := g.forward[src]
	_, inR := g.reverse[src]
	if !inF && !inR {
		return nil
	}
	prev := map[string]string{src: ""}
	queue := []string{src}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == dst {
			var chain []string
			for n := cur; n != ""; n = prev[n] {
				chain = append([]string{n}, chain...)
			}
			return chain
		}
		nbrs := map[string]bool{}
		for _, n := range g.forward[cur] {
			nbrs[n] = true
		}
		if !directed {
			for _, n := range g.reverse[cur] {
				nbrs[n] = true
			}
		}
		next := setKeys(nbrs)
		blockfiles.SortLabels(next)
		for _, n := range next {
			_, seen := prev[n]
			if _, ok := g.forward[n]; !seen && ok {
				prev[n] = cur
				queue = append(queue, n)
			}
		}
	}
	return nil
}

// lintFinding is one non-canonical citation of lint_refs.
type lintFinding struct {
	block, path, form, text, canonical string
	line                               int
}

// classifyRef is graph.classify_ref: the non-canonical form of a match, "" if accepted.
func classifyRef(m *pyre.Match, text []rune) string {
	s := m.Group(0)
	if g := m.Group(1); g != "" {
		switch {
		case strings.HasPrefix(strings.ToLower(s), "[block"):
			return "[Block N]"
		case strings.Contains(s, "#"):
			return "[Bloque #N]"
		}
		return "[Bloque N]"
	}
	if g := m.Group(2); g != "" {
		n := pyInt(g)
		if s == "[B"+n+"]" || s == "[B"+n+" §" {
			return ""
		}
		return "[B<N>] variant"
	}
	if strings.HasSuffix(string(text[:m.Start()]), "mental-model-") {
		return ""
	}
	return "bloqueN"
}

// lintRefs is graph.lint_refs over (label, path) files; unreadable paths are returned.
func lintRefs(files []blockFile) (found []lintFinding, unreadable []string) {
	for _, f := range files {
		text, ok := readText(f.path)
		if !ok {
			unreadable = append(unreadable, f.path)
			continue
		}
		rs := []rune(text)
		line, pos := 1, 0
		for _, m := range refRe.FindIter(rs) {
			form := classifyRef(m, rs)
			if form == "" {
				continue
			}
			for _, r := range rs[pos:m.Start()] {
				if r == '\n' {
					line++
				}
			}
			pos = m.Start()
			num := ""
			for g := 1; g <= 3 && num == ""; g++ {
				num = m.Group(g)
			}
			canonical := "[B" + pyInt(num) + "]"
			if strings.HasSuffix(m.Group(0), "§") {
				canonical = "[B" + pyInt(num) + " §…]"
			}
			found = append(found, lintFinding{f.label, f.path, form, m.Group(0), canonical, line})
		}
	}
	return found, unreadable
}
