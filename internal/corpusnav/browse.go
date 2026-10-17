package corpusnav

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpusnav/browse.py: show, list, by-marker, by-focus, stats and the focus and
// evidence-marker rules.

const markerWord = `(?:CERT|INFER)(?:-[\w-]+)?`

var (
	markerTokenRe = pyre.MustCompile(`\[\s*(`+markerWord+`(?:\s*[/+,]\s*`+markerWord+`)*)(?![\w-])[^\[\]]*\]`, 0)
	markerSepRe   = pyre.MustCompile(`\s*[/+,]\s*`, 0)
)

// markerComponents is browse.marker_components: every marker component, combined tokens
// ([A/B], [A+B], [A, B]) expanded.
func markerComponents(text string) []string {
	var out []string
	for _, m := range markerTokenRe.FindIter([]rune(text)) {
		for _, part := range splitRe(m.Group(1), markerSepRe) {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// splitRe is re.split(rx, s) for a pattern without groups.
func splitRe(s string, rx *pyre.Regexp) []string {
	rs := []rune(s)
	var out []string
	last := 0
	for _, m := range rx.FindIter(rs) {
		out = append(out, string(rs[last:m.Start()]))
		last = m.End()
	}
	return append(out, string(rs[last:]))
}

func fold(s string) string { return textfold.Fold(s, textfold.Corpus) }

// focusHeaderRe is browse.focus_header_re over accent-folded text.
func focusHeaderRe(focus string) *pyre.Regexp {
	return pyre.MustCompile("(?:\\*\\*)?\\bfoc(?:us|o)\\b(?:\\*\\*)?\\s*:?\\s*(?:\\*\\*)?\\s*[`*]*\\s*"+
		escape(fold(pycompat.Strip(focus)))+`(?![\w-]|\.\w)`, 0)
}

// escape is re.escape for a literal inside a pattern.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// blocksForFocus is browse.blocks_for_focus: the header tag in the first 2000 characters
// of a block, united with the blocks RESEARCH-STATE-<focus>.md cites; in label order.
func (c *corpus) blocksForFocus(focus string) []string {
	re := focusHeaderRe(focus)
	found := map[string]bool{}
	for _, b := range append(c.blocks(), c.subBlocks()...) {
		text, _ := readText(b.path)
		if rs := []rune(text); len(rs) > 2000 {
			text = string(rs[:2000])
		}
		if re.MatchString(fold(text)) {
			found[b.label] = true
		}
	}
	state := pyJoin(c.root, "RESEARCH-STATE-"+focus+".md")
	if st, err := os.Stat(state); err == nil && st.Mode().IsRegular() {
		text, _ := readText(state)
		for n := range refsInText(text, false) {
			found[n] = true
		}
	}
	labels := setKeys(found)
	sortLabels(labels)
	return labels
}

func (c *corpus) availableFocuses() []string {
	entries, _ := os.ReadDir(c.root)
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, "RESEARCH-STATE-") && strings.HasSuffix(n, ".md") &&
			len(n) >= len("RESEARCH-STATE-.md") {
			out = append(out, n[len("RESEARCH-STATE-"):len(n)-len(".md")])
		}
	}
	return out
}

func (c *corpus) cmdShow(n int) int {
	catalog := c.loadCatalog()
	b, ok := c.findBlockPath(n)
	if !ok {
		fmt.Fprintf(c.err, "Block B%d not found.\n", n)
		return 1
	}
	fmt.Fprintf(c.out, "%s · %s\n  file: %s\n", displayID(b.label), titleFor(b.label, catalog, b.path), relTo(b.path, c.root))
	fmt.Fprintln(c.out, "\n## Outline")
	text, _ := readText(b.path)
	headings := false
	for _, line := range pycompat.SplitLines(text, false) {
		switch {
		case strings.HasPrefix(line, "### "):
			fmt.Fprintf(c.out, "    - %s\n", pycompat.Strip(line[4:]))
			headings = true
		case strings.HasPrefix(line, "## "):
			fmt.Fprintf(c.out, "  - %s\n", pycompat.Strip(line[3:]))
			headings = true
		}
	}
	if !headings {
		fmt.Fprintln(c.out, "  (no sub-headings)")
	}
	fmt.Fprintln(c.out, "\n## Connections")
	conn := pycompat.Strip(strings.Join(extractSection(b.path, "Connections", "Conexiones"), "\n"))
	if conn == "" {
		fmt.Fprintln(c.out, "  (no Connections section)")
		return 0
	}
	for _, line := range pycompat.SplitLines(conn, false) {
		fmt.Fprintf(c.out, "  %s\n", line)
	}
	return 0
}

func (c *corpus) cmdList(focus string) int {
	catalog := c.loadCatalog()
	var inFocus map[string]bool
	if focus != "" {
		inFocus = map[string]bool{}
		for _, l := range c.blocksForFocus(focus) {
			inFocus[l] = true
		}
		if len(inFocus) == 0 {
			fmt.Fprintf(c.err, "No blocks found for focus '%s'.\n", focus)
			return 1
		}
	}
	var counts []int
	for i, blocks := range [][]blockFile{c.blocks(), c.subBlocks()} {
		var rows []blockFile
		for _, b := range blocks {
			if inFocus == nil || inFocus[b.label] {
				rows = append(rows, b)
			}
		}
		if i == 1 && len(rows) > 0 {
			fmt.Fprintln(c.out, "\nSub-corpus blocks (own numbering; not in CATALOG.md)")
		}
		for _, b := range rows {
			fmt.Fprintf(c.out, "%s · %s\n", displayID(b.label), titleFor(b.label, catalog, b.path))
		}
		counts = append(counts, len(rows))
	}
	suffix := ""
	if focus != "" {
		suffix = " in focus '" + focus + "'"
	}
	fmt.Fprintf(c.out, "\nTotal: %d block(s) + %d sub-corpus block(s)%s\n", counts[0], counts[1], suffix)
	return 0
}

func (c *corpus) cmdByMarker(marker string) int {
	catalog := c.loadCatalog()
	want := textfold.Lower(strings.Trim(pycompat.Strip(marker), "[]"))
	type hit struct {
		b   blockFile
		cnt int
	}
	var rows []hit
	var unreadable []string
	blocks := c.blocks()
	for _, b := range blocks {
		text, ok := readText(b.path)
		if !ok { // zero false negatives: an unreadable block is reported, never skipped
			unreadable = append(unreadable, displayID(b.label))
			continue
		}
		cnt := 0
		for _, comp := range markerComponents(text) {
			if textfold.Lower(comp) == want {
				cnt++
			}
		}
		if cnt > 0 {
			rows = append(rows, hit{b, cnt})
		}
	}
	c.reportUnreadable(unreadable)
	code := 0
	if len(unreadable) > 0 {
		code = 2
	}
	if len(rows) == 0 {
		tail := "."
		if len(unreadable) > 0 {
			tail = fmt.Sprintf(", %d unreadable.", len(unreadable))
		}
		fmt.Fprintf(c.out, "No blocks contain marker [%s]; searched %d block(s)%s\n", marker, len(blocks)-len(unreadable), tail)
		return max(code, 1)
	}
	total := 0
	for _, r := range rows {
		fmt.Fprintf(c.out, "%s · %s · %s\n", displayID(r.b.label), padLeft(fmt.Sprint(r.cnt), 4), titleFor(r.b.label, catalog, r.b.path))
		total += r.cnt
	}
	tail := "."
	if len(unreadable) > 0 {
		tail = fmt.Sprintf("; %d block(s) unreadable, not counted.", len(unreadable))
	}
	fmt.Fprintf(c.out, "\n%d block(s), %d occurrence(s) of [%s]%s\n", len(rows), total, marker, tail)
	return code
}

func (c *corpus) cmdByFocus(focus string) int {
	catalog := c.loadCatalog()
	labels := c.blocksForFocus(focus)
	if len(labels) == 0 {
		fmt.Fprintf(c.err, "No blocks found for focus '%s'.\n", focus)
		fmt.Fprintf(c.err, "Known focuses: %s\n", strings.Join(c.availableFocuses(), ", "))
		return 1
	}
	byFile := byFileOf(append(c.blocks(), c.subBlocks()...))
	for _, label := range labels {
		title := "(block file not present)"
		if p, ok := byFile[label]; ok {
			title = titleFor(label, catalog, p)
		}
		fmt.Fprintf(c.out, "%s · %s\n", displayID(label), title)
	}
	fmt.Fprintf(c.out, "\nTotal: %d block(s) in focus '%s'.\n", len(labels), focus)
	return 0
}

func (c *corpus) cmdStats() int {
	blocks := c.blocks()
	counts := map[string]int{}
	var markers, unreadable []string
	withConnections := 0
	for _, b := range blocks {
		text, ok := readText(b.path)
		if !ok {
			unreadable = append(unreadable, displayID(b.label))
			continue
		}
		for _, m := range markerComponents(text) {
			if counts[m] == 0 {
				markers = append(markers, m)
			}
			counts[m]++
		}
		if len(extractSection(b.path, "Connections", "Conexiones")) > 0 {
			withConnections++
		}
	}
	fmt.Fprintf(c.out, "Corpus totals\n  blocks : %d\n  sub-corpus blocks: %d\n", len(blocks), len(c.subBlocks()))
	fmt.Fprintf(c.out, "  docs   : %d\n  retros : %d\n", len(dirFiles(c.docs)), len(dirFiles(c.retros)))
	fmt.Fprintf(c.out, "  focuses: %d\n  blocks with a Connections section: %d\n", len(c.availableFocuses()), withConnections)
	if len(unreadable) > 0 {
		fmt.Fprintf(c.out, "  unreadable blocks (left out of every per-block count: Connections and markers): %d\n", len(unreadable))
	}
	fmt.Fprintln(c.out, "\nEvidence markers (occurrences across blocks)")
	sortStable(markers, func(a, b string) bool {
		if counts[a] != counts[b] {
			return counts[a] > counts[b]
		}
		return a < b
	})
	for _, m := range markers {
		fmt.Fprintf(c.out, "  %s %s\n", padRight(m, 18), padLeft(fmt.Sprint(counts[m]), 6))
	}
	c.reportUnreadable(unreadable)
	if len(unreadable) > 0 {
		return 2
	}
	return 0
}
