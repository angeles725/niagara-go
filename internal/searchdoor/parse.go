package searchdoor

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The row parsers of searchdoor/parse.py. Each returns (scope lines, ranked
// items); a backend answer in a shape a parser does not recognise still yields
// rows (generic, one per line): a parse miss never turns a hit into a zero.

// parser turns one backend's stdout into (scope lines, ranked items).
type parser func(string) ([]string, []Item)

const (
	snippetWidth = 110
	docMirror    = "docSource-doc" // module_nav's copy of the doc sources: listed after the real module
)

func re(pattern string) *pyre.Regexp { return pyre.MustCompile(pattern, 0) }

// match is re.match: a match anchored at the start of s (patterns here start with ^).
func match(r *pyre.Regexp, s string) *pyre.Match { return r.Search(s) }

func lines(s string) []string { return pycompat.SplitLines(s, false) }

// clip is parse.clip: whitespace runs folded to one space, cut to width
// code points with a trailing ellipsis.
func clip(text string, width int) string {
	r := []rune(strings.Join(textfold.Fields(text), " "))
	if len(r) <= width {
		return string(r)
	}
	return string(r[:width-1]) + "…"
}

func generic(out string) ([]string, []Item) {
	var items []Item
	for _, ln := range nonblank(out) {
		items = append(items, unkeyed(clip(ln, 160)))
	}
	return nil, items
}

var blockHead = re(`^(\S+) · (.*)$`)

// parseBlocks reads corpus-nav find: 'B34 · title' headers in rank order, each
// followed by indented hit lines.
func parseBlocks(out string) ([]string, []Item) {
	var items []Item
	ls := lines(out)
	for i, ln := range ls {
		m := match(blockHead, ln)
		if m == nil {
			continue
		}
		snippet := ""
		if i+1 < len(ls) && strings.HasPrefix(ls[i+1], " ") && pycompat.Strip(ls[i+1]) != "" {
			snippet = pycompat.Strip(ls[i+1])
		}
		items = append(items, keyed(m.Group(1), clip(ln, 140), clip(snippet, snippetWidth)))
	}
	if len(items) == 0 {
		return generic(out)
	}
	return nil, items
}

var docHead = re(`^\s{2}\[([^\]]+)\]\s+(.*)$`)

// parseDocs reads niagara_help find: count/by-source lines, then
// '[source] title' / 'Lnn: hit' / 'path: p' groups.
func parseDocs(out string) ([]string, []Item) {
	type doc struct{ src, title, snip, path string }
	var scope []string
	var docs []*doc
	for _, ln := range lines(out) {
		s := pycompat.Strip(ln)
		if m := match(docHead, ln); m != nil {
			docs = append(docs, &doc{src: m.Group(1), title: pycompat.Strip(m.Group(2))})
			continue
		}
		if len(docs) == 0 {
			if s != "" {
				scope = append(scope, s)
			}
			continue
		}
		cur := docs[len(docs)-1]
		switch {
		case strings.HasPrefix(s, "path:"):
			cur.path = pycompat.Strip(ln[strings.Index(ln, "path:")+len("path:"):])
		case strings.HasPrefix(s, "..."):
			// '... and N more files': the count line above already says how many
		case s != "" && cur.snip == "":
			cur.snip = s
		}
	}
	if len(docs) == 0 {
		return generic(out)
	}
	items := make([]Item, len(docs))
	for i, d := range docs {
		key := d.path
		if key == "" {
			key = d.title
		}
		items[i] = keyed(key, fmt.Sprintf("%s  [%s] %s", key, d.src, clip(d.title, 70)), clip(d.snip, snippetWidth))
	}
	return scope, items
}

// relaxed reports that niagara_help widened an AND query to 'any term'.
func relaxed(scope []string) bool {
	for _, s := range scope {
		if strings.Contains(s, "no file has every term") {
			return true
		}
	}
	return false
}

var (
	grepModule = re(`^\s{2}\[(.+)\]\s*$`)
	grepRow    = re(`^\s{4}([\w$.]+):(\d+)\s(.*)$`)
)

// parseGrep reads module_nav grep: '[module]' groups of 'Class:line text',
// ranked by query terms in the class name, then distinct terms on its matched
// lines, then matched lines, then first appearance.
func parseGrep(out string, terms []string) ([]string, []Item) {
	type class struct {
		name, module, snip string
		n, at              int
		terms              map[string]bool
	}
	var scope []string
	var found []*class
	index := map[[2]string]*class{}
	module, haveModule := "", false
	for _, ln := range lines(out) {
		if m := match(grepModule, ln); m != nil {
			module, haveModule = m.Group(1), true
			continue
		}
		m := match(grepRow, ln)
		if m != nil && haveModule && module != "" {
			k := [2]string{m.Group(1), module}
			c := index[k]
			if c == nil {
				c = &class{name: k[0], module: module, snip: m.Group(3), at: len(found), terms: map[string]bool{}}
				index[k] = c
				found = append(found, c)
			}
			c.n++
			text := textfold.Lower(m.Group(3))
			for _, t := range terms {
				if strings.Contains(text, t) {
					c.terms[t] = true
				}
			}
		} else if pycompat.Strip(ln) != "" && len(found) == 0 && len(scope) == 0 {
			scope = append(scope, pycompat.Strip(ln))
		}
	}
	if len(found) == 0 {
		return generic(out)
	}
	inName := func(c *class) int {
		n, name := 0, textfold.Lower(c.name)
		for _, t := range terms {
			if strings.Contains(name, t) {
				n++
			}
		}
		return n
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if x, y := inName(a), inName(b); x != y {
			return x > y
		}
		if len(a.terms) != len(b.terms) {
			return len(a.terms) > len(b.terms)
		}
		if a.n != b.n {
			return a.n > b.n
		}
		return a.at < b.at
	})
	items := make([]Item, len(found))
	for i, c := range found {
		items[i] = keyed(c.name, fmt.Sprintf("%s  [%s]  %d/%d terms, %d lines", c.name, c.module, len(c.terms),
			len(terms), c.n), clip(c.snip, snippetWidth))
	}
	return scope, items
}

var rankRow = re(`^(\d+)\t([\d.]+)\t(\S+)\t(\S+)\t(.*)$`)

// parseRank reads module_nav rank: a 'Rank ...: N matching classes' header,
// then tab-separated rows 'rank score class module path', best first.
// rankTotalRe is the class count of module_nav rank's header line
// ("Rank 'q': 1,302 matching classes (terms: ...)").
var rankTotalRe = re(`: ([\d,]+) matching classes\b`)

// rankTotal is the class count of the rank header among scope lines, 0 when absent.
func rankTotal(scope []string) int {
	for _, ln := range scope {
		if m := rankTotalRe.Search(ln); m != nil {
			n, _ := strconv.Atoi(strings.ReplaceAll(m.Group(1), ",", ""))
			return n
		}
	}
	return 0
}

// keyedCount is the number of items a parser recognised (rank rows, not its fallback lines).
func keyedCount(items []Item) int {
	n := 0
	for _, it := range items {
		if it.Keyed {
			n++
		}
	}
	return n
}

func parseRank(out string) ([]string, []Item) {
	var scope []string
	for _, ln := range nonblank(out) {
		if s := pycompat.Strip(ln); strings.HasPrefix(s, "Rank ") {
			scope = []string{s}
			break
		}
	}
	var items []Item
	for _, ln := range lines(out) {
		if m := match(rankRow, ln); m != nil {
			name := m.Group(3)
			items = append(items, keyed(name, fmt.Sprintf("%s  [%s]  score %s", name, m.Group(4), m.Group(2)),
				clip(m.Group(5), snippetWidth)))
		}
	}
	if len(items) == 0 {
		return generic(out)
	}
	return scope, items
}

// merge concatenates ranked item groups, keeping the first item per key
// (rows without a key all stay).
func merge(groups ...[]Item) []Item {
	seen := map[string]bool{}
	var out []Item
	for _, g := range groups {
		for _, it := range g {
			if it.Keyed && seen[it.Key] {
				continue
			}
			if it.Keyed {
				seen[it.Key] = true
			}
			out = append(out, it)
		}
	}
	return out
}

var tokenRow = re(`^\s{2}([\w$.]+)\s+(\d+)\s+(\S+)\s*$`)

// parseTokens reads module_nav token, one output per identifier: occurrence
// rows 'Class  line  module'. Classes named exactly like a queried identifier
// first, then by occurrences; the doc mirror module listed last.
func parseTokens(outs, idents []string) ([]string, []Item) {
	type class struct {
		n    int
		mods []string
	}
	var scope, order []string
	found := map[string]*class{}
	for _, out := range outs {
		for _, ln := range nonblank(out) {
			if s := pycompat.Strip(ln); strings.HasPrefix(s, "TOKEN:") {
				scope = append(scope, s)
			}
		}
		for _, ln := range lines(out) {
			m := match(tokenRow, ln)
			if m == nil {
				continue
			}
			c := found[m.Group(1)]
			if c == nil {
				c = &class{}
				found[m.Group(1)] = c
				order = append(order, m.Group(1))
			}
			c.n++
			if !slices.Contains(c.mods, m.Group(3)) {
				c.mods = append(c.mods, m.Group(3))
			}
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if x, y := slices.Contains(idents, a), slices.Contains(idents, b); x != y {
			return x
		}
		if found[a].n != found[b].n {
			return found[a].n > found[b].n
		}
		return a < b
	})
	items := make([]Item, len(order))
	for i, name := range order {
		mods := append([]string(nil), found[name].mods...)
		sort.SliceStable(mods, func(i, j int) bool { return mods[i] != docMirror && mods[j] == docMirror })
		shown, more := mods, ""
		if len(mods) > 3 {
			shown, more = mods[:3], ", …"
		}
		items[i] = keyed(name, fmt.Sprintf("%s  [%s%s]  %d occurrences", name, strings.Join(shown, ", "), more,
			found[name].n), "")
	}
	return scope, items
}

// parseLicense reads module_nav license-feature / license-features: a header
// line, then table or feature rows.
func parseLicense(out string) ([]string, []Item) {
	ls := nonblank(out)
	if len(ls) == 0 {
		return nil, nil
	}
	var items []Item
	for _, ln := range ls[1:] {
		r := pycompat.Strip(ln)
		if strings.Trim(r, "-") == "" || strings.HasPrefix(r, "CLASS ") || strings.HasPrefix(r, "FEATURE ") ||
			strings.HasPrefix(r, "Try:") {
			continue
		}
		items = append(items, keyed(textfold.Fields(r)[0], clip(r, 170), ""))
	}
	return []string{pycompat.Strip(ls[0])}, items
}

var moduleRow = re(`^\s{2}(\S+)\s+\S+\s+[\d,]+\s+[\d,]+`)

// moduleNames reads module_nav modules: 'NAME TYPE JAVA CLASS ...' rows.
func moduleNames(out string) map[string]bool {
	names := map[string]bool{}
	for _, ln := range lines(out) {
		if m := match(moduleRow, ln); m != nil {
			names[m.Group(1)] = true
		}
	}
	return names
}
