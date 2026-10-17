package help

// Import cross-references over xref-index.json (xref_search.py).

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/index"
)

// XrefParams are the arguments of `xref`.
type XrefParams struct {
	Name      string
	Direction string // "uses", "used_by" or "both"
}

// Xref shows what a class imports (uses) and what imports it (used_by),
// matching the name exactly or else case-insensitively (cmd_xref).
func (e *Env) Xref(p XrefParams) error {
	// xref_search reads the file directly: a missing index is an error, not
	// an empty scope.
	idx, err := e.loadIndex("xref-index.json", true)
	if err != nil {
		return err
	}
	name := p.Name
	if !idx.Has(name) {
		found := false
		for _, k := range idx.Keys() {
			if lowerStr(k) == lowerStr(name) {
				name, found = k, true
				break
			}
		}
		if !found {
			return e.reportMiss("xref", p.Name, "xref")
		}
	}
	entry := idx.Object(name)
	fmt.Fprintf(e.Out, "XREF: %s (%s)\n", name, orStr(entry, "package", "?", "default package"))
	if p.Direction == "both" || p.Direction == "uses" {
		e.printRefs(idx, "USES", strList(entry, "uses"))
	}
	if p.Direction == "both" || p.Direction == "used_by" {
		e.printRefs(idx, "USED BY", strList(entry, "used_by"))
	}
	return nil
}

func (e *Env) printRefs(idx *index.Object, title string, classes []string) {
	fmt.Fprintf(e.Out, "\n%s (%d classes):\n", title, len(classes))
	if len(classes) == 0 {
		fmt.Fprintln(e.Out, "  (none found in source/)")
		return
	}
	for _, cls := range classes {
		pkg := ""
		if v := get(getObj(idx, cls), "package", ""); truthy(v) {
			pkg = "(" + pyStr(v) + ")"
		}
		fmt.Fprintf(e.Out, "  %s %s\n", cls, pkg)
	}
}
