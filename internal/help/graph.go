package help

// ASCII dependency graph of one class (graph_search.py) and the corpus
// hotspots (hotspots_search.py).

import (
	"fmt"
	"sort"
	"strings"
)

// pyHead is items[:n] with Python's slice bounds: a negative n counts from
// the end, and the result is clamped to the list.
func pyHead[T any](items []T, n int) []T {
	if n < 0 {
		n += len(items)
	}
	return items[:max(0, min(n, len(items)))]
}

// GraphParams are the arguments of `graph`.
type GraphParams struct {
	Name     string
	MaxNodes int
}

// Graph is cmd_graph: the class's hierarchy, then what it imports on the
// left and what imports it on the right.
func (e *Env) Graph(p GraphParams) error {
	xref, err := e.loadIndex("xref-index.json", true)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", true)
	if err != nil {
		return err
	}
	name, ok := resolveClassKey(xref, p.Name)
	if !ok {
		return e.reportMiss("graph", p.Name, "xref")
	}
	entry, n := xref.Object(name), p.MaxNodes
	uses, usedBy := strList(entry, "uses"), strList(entry, "used_by")
	sort.Strings(uses)
	sort.Strings(usedBy)
	ih := getObj(inh, name)
	parent, impls, subs := truthyStr(ih, "extends"), strList(ih, "implements"), strList(ih, "subclasses")
	sort.Strings(subs)
	center := "[ " + name + " ]"
	out := e.Out
	fmt.Fprintf(out, "\nDEPENDENCY GRAPH: %s\n%s\n", name, strings.Repeat("=", 60))
	if parent != "" || len(impls) > 0 || len(subs) > 0 {
		fmt.Fprint(out, "\nHIERARCHY:\n")
		if parent != "" {
			fmt.Fprintf(out, "  %s <-- extends\n", parent)
		}
		fmt.Fprintf(out, "    |\n    %s\n", center)
		for _, i := range impls {
			fmt.Fprintf(out, "    |   implements --> %s\n", i)
		}
		shown := pyHead(subs, n)
		for i, s := range shown {
			conn := "|--"
			if i == len(shown)-1 {
				conn = "`--"
			}
			fmt.Fprintf(out, "    %s  %s\n", conn, s)
		}
		if len(subs) > 0 && len(subs) > n {
			fmt.Fprintf(out, "    ... +%d more\n", len(subs)-n)
		}
	}
	fmt.Fprint(out, "\nIMPORTS:\n")
	left, right := pyHead(uses, n), pyHead(usedBy, n)
	width := 10
	for _, u := range left {
		width = max(width, runeLen(u))
	}
	if len(uses) == 0 && len(usedBy) == 0 {
		fmt.Fprintln(out, "  (no import relationships found)")
		return nil
	}
	fmt.Fprintf(out, "  %s    %s    %s\n", rjust(fmt.Sprintf("USES (%d)", len(uses)), width), center,
		fmt.Sprintf("USED BY (%d)", len(usedBy)))
	fmt.Fprintf(out, "  %s    %s    %s\n", rjust(strings.Repeat("-", min(width, 30)), width),
		strings.Repeat("-", runeLen(center)), strings.Repeat("-", 30))
	for i := range max(len(left), len(right)) {
		l, r := "", ""
		arrowL, arrowR, mid := "    ", "    ", strings.Repeat(" ", runeLen(center))
		if i < len(left) {
			l, arrowL = left[i], " -->"
		}
		if i < len(right) {
			r, arrowR = right[i], "<-- "
		}
		if i == 0 {
			mid = center
		}
		fmt.Fprintf(out, "  %s %s %s %s %s\n", rjust(l, width), arrowL, mid, arrowR, r)
	}
	if len(uses) > n {
		fmt.Fprintf(out, "  %s      ... +%d more\n", rjust("", width), len(uses)-n)
	}
	if len(usedBy) > n {
		fmt.Fprintf(out, "  %s%s... +%d more\n", rjust("", width), strings.Repeat(" ", runeLen(center)+10), len(usedBy)-n)
	}
	fmt.Fprintf(out, "\nTotal: %d uses, %d used-by\n", len(uses), len(usedBy))
	return nil
}

type hotspot struct {
	count     int
	name, pkg string
}

// Hotspots is cmd_hotspots: the classes most imported, the classes
// importing most and the most common property types, top n of each.
func (e *Env) Hotspots(top int) error {
	xref, err := e.loadIndex("xref-index.json", true)
	if err != nil {
		return err
	}
	slots, err := e.loadIndex("slots-index.json", true)
	if err != nil {
		return err
	}
	if xref.Len() == 0 {
		return e.reportMiss("hotspots", "", "xref")
	}
	rank := func(key string) []hotspot {
		var hs []hotspot
		for _, cls := range xref.Keys() {
			d := xref.Object(cls)
			if n := listLen(d, key); n > 0 {
				hs = append(hs, hotspot{n, cls, getStr(d, "package", "")})
			}
		}
		sort.SliceStable(hs, func(i, j int) bool { return hs[i].count > hs[j].count })
		return hs
	}
	var types counter
	for _, cls := range slots.Keys() {
		for _, p := range objList(slots.Object(cls), "properties") {
			if t := truthyStr(p, "type"); t != "" {
				types.add(t)
			}
		}
	}
	out := e.Out
	fmt.Fprintf(out, "HOTSPOTS ANALYSIS\n%s\n", strings.Repeat("=", 60))
	e.hotspotTable(fmt.Sprintf("MOST DEPENDED ON (top %d by used_by count)", top), "refs", rank("used_by"), top)
	e.hotspotTable(fmt.Sprintf("MOST COMPLEX (top %d by uses count)", top), "deps", rank("uses"), top)
	if all := types.byCount(); len(all) > 0 {
		fmt.Fprintf(out, "\nMOST USED SLOT TYPES (top %d):\n\n", top)
		fmt.Fprintf(out, "  count  type\n  -----  ----\n")
		for _, t := range pyHead(all, top) {
			fmt.Fprintf(out, "  %5d  %s\n", t.count, t.name)
		}
	}
	edges := 0
	for _, cls := range xref.Keys() {
		edges += listLen(xref.Object(cls), "uses")
	}
	fmt.Fprintf(out, "\nTOTALS: %d classes, %s import edges, %d slot types\n", xref.Len(), thousands(edges), len(types.order))
	return nil
}

func (e *Env) hotspotTable(title, label string, hs []hotspot, top int) {
	fmt.Fprintf(e.Out, "\n%s:\n\n", title)
	fmt.Fprintf(e.Out, "  %s  %s  package\n", rjust(label, 5), ljust("class", 40))
	fmt.Fprintf(e.Out, "  %s  %s  -------\n", rjust("----", 5), ljust("-----", 40))
	for _, h := range pyHead(hs, top) {
		name := h.name
		if runeLen(name) > 40 {
			name = headRunes(name, 37) + "..."
		}
		fmt.Fprintf(e.Out, "  %5d  %s  %s\n", h.count, ljust(name, 40), h.pkg)
	}
}
