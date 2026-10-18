package help

// Member lookup over method-index.json (method_search.cmd_method): method
// by exact, case-insensitive or glob name, entries ordered by relevance.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// MethodParams are the arguments of `method`.
type MethodParams struct {
	Name    string
	Section string // "" for every section
}

// memberEntries is index[name]: the entry objects of one member name.
func memberEntries(idx *index.Object, name string) []*index.Object {
	arr, _ := get(idx, name, nil).([]any)
	out := make([]*index.Object, 0, len(arr))
	for _, v := range arr {
		o, _ := v.(*index.Object)
		out = append(out, o)
	}
	return out
}

// memberMatches is method_search.member_matches: the entries of every member
// named name, exactly, else case-insensitively (keys in index order).
func memberMatches(idx *index.Object, lower map[string][]string, name string) []*index.Object {
	if idx.Has(name) {
		return memberEntries(idx, name)
	}
	var out []*index.Object
	for _, k := range lower[pycompat.Lower(name)] {
		out = append(out, memberEntries(idx, k)...)
	}
	return out
}

// lowerKeys is _lower_keys: lower-case key -> keys, in index order.
func lowerKeys(idx *index.Object) map[string][]string {
	m := map[string][]string{}
	for _, k := range idx.Keys() {
		l := pycompat.Lower(k)
		m[l] = append(m[l], k)
	}
	return m
}

// inSection is the --section filter of cmd_method.
func inSection(entries []*index.Object, section string) []*index.Object {
	if section == "" {
		return entries
	}
	var out []*index.Object
	for _, e := range entries {
		if getStr(e, "section", "") == section {
			out = append(out, e)
		}
	}
	return out
}

var genericTail = regexp.MustCompile(`&lt;.*|<.*`)

// memberClass is _class_name: the class of an entry without its type
// parameters.
func memberClass(e *index.Object) string {
	return genericTail.ReplaceAllString(orStr(e, "class", "", ""), "")
}

// sectionOrder is SECTION_ORDER (3 for any other section).
func sectionOrder(s string) int {
	switch s {
	case "methods":
		return 0
	case "fields":
		return 1
	case "constructors":
		return 2
	}
	return 3
}

// docFrequency is fulltext.doc_frequency: for each word the number of
// indexed documents containing it as a whole word. A missing or stale
// full-text index is rebuilt first; when none can be built every count is 0
// and a note goes to stderr.
func (e *Env) docFrequency(words []string) map[string]int {
	if len(words) == 0 {
		return map[string]int{}
	}
	wanted := map[string]string{}
	var terms []string
	for _, w := range words {
		t := ""
		if wt := fts.WordTerms(w); len(wt) > 0 {
			t = wt[0]
		}
		wanted[w] = t
		if t != "" {
			terms = append(terms, t)
		}
	}
	counts, err := e.docCounts(terms)
	if err != nil {
		fmt.Fprintf(e.Err, "full-text index unavailable (%v); results are not ordered by doc prominence\n", err)
		counts = map[string]int{}
	}
	out := map[string]int{}
	for w, t := range wanted {
		out[w] = counts[t]
	}
	return out
}

func (e *Env) docCounts(terms []string) (map[string]int, error) {
	x, err := scope.OpenFulltext(e.Err)(e.BaseDir, e.VersionID)
	if err != nil {
		return nil, err
	}
	defer x.Close()
	return x.DocCounts(terms)
}

// byRelevance is method_search.by_relevance: entries whose class or package
// shares a word with the member first, then the classes the docs mention
// most, then class, package and section.
func (e *Env) byRelevance(member string, entries []*index.Object) []*index.Object {
	memberWords := map[string]bool{}
	for _, t := range fts.WordTerms(member) {
		memberWords[t] = true
	}
	seen := map[string]bool{}
	var classes []string
	for _, en := range entries {
		if c := memberClass(en); c != "" && !seen[c] {
			seen[c] = true
			classes = append(classes, c)
		}
	}
	sort.Strings(classes)
	prominence := e.docFrequency(classes)
	type keyed struct {
		shared, prom, section int
		cls, pkg              string
		entry                 *index.Object
	}
	ks := make([]keyed, len(entries))
	for i, en := range entries {
		cls := memberClass(en)
		pkg := orStr(en, "package", "", "")
		shared := 0
		for t := range fts.TermSet(cls + " " + pkg) {
			if memberWords[t] {
				shared++
			}
		}
		ks[i] = keyed{shared, prominence[cls], sectionOrder(getStr(en, "section", "None")), cls, pkg, en}
	}
	sort.SliceStable(ks, func(i, j int) bool {
		a, b := ks[i], ks[j]
		switch {
		case a.shared != b.shared:
			return a.shared > b.shared
		case a.prom != b.prom:
			return a.prom > b.prom
		case a.cls != b.cls:
			return a.cls < b.cls
		case a.pkg != b.pkg:
			return a.pkg < b.pkg
		}
		return a.section < b.section
	})
	out := make([]*index.Object, len(ks))
	for i, k := range ks {
		out[i] = k.entry
	}
	return out
}

// Method is method_search.cmd_method.
func (e *Env) Method(p MethodParams) error {
	idx, err := e.loadIndex("method-index.json", false)
	if err != nil {
		return err
	}
	if idx.Has(p.Name) {
		entries := inSection(memberEntries(idx, p.Name), p.Section)
		return e.printMemberResults(p.Name, e.byRelevance(p.Name, entries), p.Section)
	}
	if fold := lowerKeys(idx)[pycompat.Lower(p.Name)]; len(fold) > 0 {
		for _, k := range fold {
			entries := inSection(memberEntries(idx, k), p.Section)
			if err := e.printMemberResults(k, e.byRelevance(k, entries), p.Section); err != nil {
				return err
			}
		}
		return nil
	}
	var names []string
	for _, k := range idx.Keys() {
		if fnmatch.Fnmatch(k, p.Name) {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		for _, k := range idx.Keys() {
			if lowerGlob(k, p.Name) {
				names = append(names, k)
			}
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return e.reportMiss("method", p.Name, "methods")
	}
	total := 0
	for _, n := range names {
		total += len(memberEntries(idx, n))
	}
	fmt.Fprintf(e.Out, "%d member names matching '%s' (%d total entries):\n\n", len(names), p.Name, total)
	for _, n := range pyHead(names, 30) {
		entries := inSection(memberEntries(idx, n), p.Section)
		if len(entries) == 0 {
			continue
		}
		seen := map[string]bool{}
		var classes []string
		for _, en := range entries {
			if c := orStr(en, "class", "", ""); c != "" && !seen[c] {
				seen[c] = true
				classes = append(classes, c)
			}
		}
		sort.Strings(classes)
		fmt.Fprintf(e.Out, "  %s (%d classes)\n", n, len(classes))
		fmt.Fprintf(e.Out, "    e.g.: %s\n", getStr(entries[0], "signature", "None"))
		if len(classes) <= 5 {
			fmt.Fprintf(e.Out, "    in: %s\n", strings.Join(classes, ", "))
		} else {
			fmt.Fprintf(e.Out, "    in: %s ... +%d more\n", strings.Join(classes[:5], ", "), len(classes)-5)
		}
	}
	if len(names) > 30 {
		fmt.Fprintf(e.Out, "\n  ... and %d more names\n", len(names)-30)
	}
	return nil
}

// printMemberResults is _print_results: every entry of one member name, or
// the miss of its --section filter.
func (e *Env) printMemberResults(name string, entries []*index.Object, section string) error {
	if len(entries) == 0 {
		sec := section
		if sec == "" {
			sec = "None"
		}
		return e.reportMiss("method --section "+sec, name, "methods")
	}
	fmt.Fprintf(e.Out, "'%s' found in %d classes:\n\n", name, len(entries))
	for _, en := range entries {
		fmt.Fprintf(e.Out, "  [%s] %s.%s\n", getStr(en, "section", "None"), getStr(en, "package", "?"),
			getStr(en, "class", "?"))
		fmt.Fprintf(e.Out, "    %s\n", getStr(en, "signature", "None"))
	}
	return nil
}
