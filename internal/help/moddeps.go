package help

// Module dependency graph from source-index.json (class -> module) and
// xref-index.json (class -> imported classes) (module_deps.py).

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// sourceModules groups source-index.json by module: the modules in first-seen
// order, their classes in index order, and the module of every class.
type sourceModules struct {
	order    []string
	classes  map[string][]namedEntry
	moduleOf map[string]string
}

func newSourceModules(src *index.Object) *sourceModules {
	m := &sourceModules{classes: map[string][]namedEntry{}, moduleOf: map[string]string{}}
	for _, cls := range src.Keys() {
		d := src.Object(cls)
		mod := getStr(d, "module", "")
		if _, seen := m.classes[mod]; !seen {
			m.order = append(m.order, mod)
		}
		m.classes[mod] = append(m.classes[mod], namedEntry{cls, d})
		m.moduleOf[cls] = mod
	}
	return m
}

// partial lists the modules whose name contains name, case-insensitively.
func (m *sourceModules) partial(name string) []string {
	var out []string
	for _, mod := range m.order {
		if strings.Contains(lowerStr(mod), lowerStr(name)) {
			out = append(out, mod)
		}
	}
	return out
}

// printList prints "  <module> (N classes)" for the given modules, sorted.
func (m *sourceModules) printList(e *Env, mods []string) {
	mods = append([]string(nil), mods...)
	sort.Strings(mods)
	for _, mod := range mods {
		fmt.Fprintf(e.Out, "  %s (%d classes)\n", mod, len(m.classes[mod]))
	}
}

// pyInt is a decoded JSON integer as a Go int (0 for anything else).
func pyInt(v any) int {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

// ModuleDepsParams are the arguments of `module-deps`.
type ModuleDepsParams struct {
	Name    string
	Reverse bool
}

// ModuleDeps is cmd_module_deps: the modules a module imports from, or with
// Reverse the modules importing from it, with up to five example classes.
func (e *Env) ModuleDeps(p ModuleDepsParams) error {
	src, err := e.loadIndex("source-index.json", true)
	if err != nil {
		return err
	}
	xref, err := e.loadIndex("xref-index.json", true)
	if err != nil {
		return err
	}
	m := newSourceModules(src)
	name := p.Name
	if _, ok := m.classes[name]; !ok {
		matches := m.partial(name)
		switch len(matches) {
		case 0:
			if err := e.reportMiss("module-deps", name, "modules"); err != nil {
				return err
			}
			fmt.Fprintln(e.Out, "\nAvailable modules:")
			m.printList(e, m.order)
			return nil
		case 1:
			name = matches[0]
		default:
			fmt.Fprintf(e.Out, "%d modules match '%s':\n", len(matches), name)
			m.printList(e, matches)
			return nil
		}
	}
	mine := map[string]bool{}
	for _, c := range m.classes[name] {
		mine[c.name] = true
	}
	// deps: other module -> the example class of each import edge
	deps := map[string][]string{}
	if !p.Reverse {
		for _, c := range m.classes[name] {
			for _, used := range strList(getObj(xref, c.name), "uses") {
				if other := m.moduleOf[used]; other != "" && other != name {
					deps[other] = append(deps[other], used)
				}
			}
		}
	} else {
		for _, cls := range xref.Keys() {
			mod := m.moduleOf[cls]
			if mod == "" || mod == name {
				continue
			}
			for _, used := range strList(xref.Object(cls), "uses") {
				if mine[used] {
					deps[mod] = append(deps[mod], cls)
				}
			}
		}
	}
	title, arrow := "DEPENDS ON", "<-"
	if p.Reverse {
		title, arrow = "DEPENDED ON BY", "->"
	}
	fmt.Fprintf(e.Out, "MODULE: %s (%d classes)\n\n%s (%d modules):\n\n", name, len(mine), title, len(deps))
	for _, mod := range sortedKeys(deps) {
		examples := sortedUnique(deps[mod])
		fmt.Fprintf(e.Out, "  %s (%d imports)\n", mod, len(deps[mod]))
		for _, cls := range examples[:min(len(examples), 5)] {
			fmt.Fprintf(e.Out, "    %s %s\n", arrow, cls)
		}
		if len(examples) > 5 {
			fmt.Fprintf(e.Out, "    ... +%d more\n", len(examples)-5)
		}
	}
	if len(deps) == 0 {
		fmt.Fprintln(e.Out, "  (none found in source corpus)")
	}
	return nil
}

func sortedUnique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
