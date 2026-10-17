package help

// Orphan detection (interface_hierarchy_search.cmd_orphans): classes no
// other indexed class imports.

import (
	"fmt"
	"sort"
	"strings"
)

type orphan struct {
	name, pkg  string
	uses, subs int
}

// OrphansParams are the arguments of `orphans`.
type OrphansParams struct{ Top int }

// Orphans lists the classes with no used_by reference: xref entries with an
// empty used_by plus class-index classes absent from the xref index, isolated
// ones (no subclasses) first, at most Top of each group.
func (e *Env) Orphans(p OrphansParams) error {
	xref, err := e.loadIndex("xref-index.json", true)
	if err != nil {
		return err
	}
	ci, err := e.loadIndex("class-index.json", true)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", true)
	if err != nil {
		return err
	}
	if xref.Len() == 0 {
		return e.reportMiss("orphans", "", "xref")
	}
	var all []orphan
	for _, cls := range xref.Keys() {
		data := xref.Object(cls)
		if listLen(data, "used_by") == 0 {
			all = append(all, orphan{cls, getStr(getObj(ci, cls), "package", ""),
				listLen(data, "uses"), listLen(getObj(inh, cls), "subclasses")})
		}
	}
	for _, cls := range ci.Keys() {
		if !xref.Has(cls) {
			all = append(all, orphan{cls, getStr(ci.Object(cls), "package", ""),
				0, listLen(getObj(inh, cls), "subclasses")})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].subs != all[j].subs {
			return all[i].subs < all[j].subs
		}
		return all[i].name < all[j].name
	})
	fmt.Fprintf(e.Out, "ORPHAN CLASSES (0 used_by references)\n%s\n\n", strings.Repeat("=", 65))
	fmt.Fprintf(e.Out, "Total: %d orphans out of %d indexed classes\n\n", len(all), ci.Len())
	var isolated, roots []orphan
	for _, o := range all {
		if o.subs == 0 {
			isolated = append(isolated, o)
		} else {
			roots = append(roots, o)
		}
	}
	fmt.Fprintf(e.Out, "ISOLATED (no references, no subclasses): %d\n\n", len(isolated))
	fmt.Fprintf(e.Out, "  %s  %s  %s\n", ljust("class", 40), rjust("uses", 5), "package")
	fmt.Fprintf(e.Out, "  %s  %s  %s\n", ljust("-----", 40), rjust("----", 5), "-------")
	for _, o := range isolated[:clampTop(p.Top, len(isolated))] {
		fmt.Fprintf(e.Out, "  %s  %s  %s\n", ljust(shortName(o.name), 40), rjust(fmt.Sprint(o.uses), 5), o.pkg)
	}
	if len(isolated) > p.Top {
		fmt.Fprintf(e.Out, "  ... +%d more\n", len(isolated)-p.Top)
	}
	if len(roots) == 0 {
		return nil
	}
	fmt.Fprintf(e.Out, "\nROOT ORPHANS (no references but have subclasses): %d\n\n", len(roots))
	fmt.Fprintf(e.Out, "  %s  %s  %s  %s\n", ljust("class", 40), rjust("uses", 5), rjust("subs", 5), "package")
	fmt.Fprintf(e.Out, "  %s  %s  %s  %s\n", ljust("-----", 40), rjust("----", 5), rjust("----", 5), "-------")
	for _, o := range roots[:clampTop(p.Top, len(roots))] {
		fmt.Fprintf(e.Out, "  %s  %s  %s  %s\n", ljust(shortName(o.name), 40),
			rjust(fmt.Sprint(o.uses), 5), rjust(fmt.Sprint(o.subs), 5), o.pkg)
	}
	if len(roots) > p.Top {
		fmt.Fprintf(e.Out, "  ... +%d more\n", len(roots)-p.Top)
	}
	return nil
}

// shortName cuts a class name to 40 code points ("..." after 37).
func shortName(s string) string {
	if runeLen(s) <= 40 {
		return s
	}
	return headRunes(s, 37) + "..."
}

// clampTop is the end index of the Python slice [:top] over n items; a
// negative top counts from the end like Python.
func clampTop(top, n int) int {
	if top < 0 {
		return max(0, n+top)
	}
	return min(top, n)
}
