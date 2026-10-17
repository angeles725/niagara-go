package help

// Discovery of related classes (related_search.py): same package, hierarchy
// (parent, interfaces, children, siblings), import dependencies and guide
// folders.

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Related is cmd_related; its indexes are optional (a missing one is empty).
func (e *Env) Related(name string) error {
	const max = 10
	classes, err := e.loadIndex("class-index.json", false)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	xref, err := e.loadIndex("xref-index.json", false)
	if err != nil {
		return err
	}
	guides, err := e.loadIndex("guides-index.json", false)
	if err != nil {
		return err
	}
	hits, err := classMatches(classes, name)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		return e.reportMiss("related", name, "classes")
	}
	name, pkg := hits[0].Name, hits[0].Package
	out := e.Out
	fmt.Fprintf(out, "RELATED: %s (%s, %s)\n%s\n", name, pkg, hits[0].Type, strings.Repeat("=", 60))

	var same []string
	for _, k := range classes.Keys() {
		if getStr(classes.Object(k), "package", "") == pkg && k != name {
			same = append(same, k)
		}
	}
	sort.Strings(same)
	fmt.Fprintf(out, "\nSAME PACKAGE - %s (%d classes):\n", pkg, len(same))
	for _, k := range same[:min(len(same), max)] {
		fmt.Fprintf(out, "  %s (%s)\n", k, getStr(classes.Object(k), "type", ""))
	}
	if len(same) > max {
		fmt.Fprintf(out, "  ... +%d more\n", len(same)-max)
	}

	inhKey := ""
	for _, k := range inh.Keys() {
		if stripGenerics(k) == name {
			inhKey = k
			break
		}
	}
	if inhKey != "" {
		entry := inh.Object(inhKey)
		parent := truthyStr(entry, "extends")
		var siblings []string
		if parent != "" {
			for _, k := range inh.Keys() {
				s := stripGenerics(k)
				if v, ok := get(inh.Object(k), "extends", nil).(string); ok && v == parent && s != name {
					siblings = append(siblings, s)
				}
			}
		}
		children, impls := strList(entry, "subclasses"), strList(entry, "implements")
		fmt.Fprintln(out, "\nHIERARCHY:")
		if parent != "" {
			fmt.Fprintf(out, "  parent: %s\n", parent)
		}
		if len(impls) > 0 {
			fmt.Fprintf(out, "  implements: %s\n", strings.Join(impls, ", "))
		}
		e.relatedList("  children", children, max)
		sort.Strings(siblings)
		e.relatedList("  siblings", siblings, max)
	}

	if xref.Has(name) {
		x := xref.Object(name)
		fmt.Fprintln(out, "\nIMPORT DEPENDENCIES:")
		e.relatedList("  uses", strList(x, "uses"), max)
		e.relatedList("  used by", strList(x, "used_by"), max)
	}

	term := lowerStr(name)
	// Python tests isupper() on the lower-cased name, so the B prefix is
	// stripped only when lowering leaves an upper-case second letter.
	if r := []rune(term); len(r) > 1 && r[0] == 'b' && unicode.IsUpper(r[1]) {
		term = string(r[1:])
	}
	var matched []string
	for _, folder := range guides.Keys() {
		if strings.Contains(lowerStr(folder), term) {
			matched = append(matched, folder)
			continue
		}
		for _, topic := range strList(guides.Object(folder), "topics") {
			if strings.Contains(lowerStr(topic), term) {
				matched = append(matched, folder)
				break
			}
		}
	}
	if len(matched) > 0 {
		fmt.Fprintf(out, "\nRELATED GUIDES (%d folders):\n", len(matched))
		folders := sortedUnique(matched)
		for _, f := range folders[:min(len(folders), max)] {
			fmt.Fprintf(out, "  %s (%s topics)\n", f, topicCount(guides.Object(f), "?"))
		}
	}
	return nil
}

// relatedList prints "<label> (N): a, b, ..." with at most max names and a
// "    ... +K more" line; nothing for an empty list.
func (e *Env) relatedList(label string, names []string, max int) {
	if len(names) == 0 {
		return
	}
	fmt.Fprintf(e.Out, "%s (%d): %s\n", label, len(names), strings.Join(names[:min(len(names), max)], ", "))
	if len(names) > max {
		fmt.Fprintf(e.Out, "    ... +%d more\n", len(names)-max)
	}
}
