package structure

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The library half of the hierarchy commands (decision D29): Subclasses,
// Chain and Implementors take the decoded inheritance.json (inh) and the
// class index "classes" Table (classes, nil when absent), print nothing and
// return typed rows; FormatTreeRow, FormatChain and FormatImplementor yield
// the exact lines `hierarchy` and `implementors` print, so the commands and
// nsearch show the same text. Loading the indexes, their stderr and the miss
// reports belong to the commands (internal/nav/structure).

// ClassRow is a class with the module and package of its top-level entry: a
// runtime copy wins over the docSource-doc copy, an -rt module first, noted
// "(+doc source)". Module and Package are "?" when Located is false (the
// class is not in the class index).
type ClassRow struct {
	Class, Module, Package string
	Located                bool
}

func classRow(name string, classes *index.Object) ClassRow {
	mod, pkg, ok := moduleInfo(name, classes)
	if !ok {
		mod, pkg = "?", "?"
	}
	return ClassRow{Class: name, Module: mod, Package: pkg, Located: ok}
}

// TreeRow is one line of a subclass tree: a class (Class set) or, where the
// depth limit cuts a class with children, the count of its Hidden
// descendants (Class empty) and the --depth (Expand) that shows them.
type TreeRow struct {
	Class   string
	Module  string // the class's module, when Located
	Located bool
	Depth   int    // 0 for the root
	Indent  string // the prefix and connector the line starts with
	Hidden  int
	Expand  int
}

// FormatTreeRow is the tree line `hierarchy` prints (no newline).
func FormatTreeRow(r TreeRow) string {
	if r.Class == "" {
		return fmt.Sprintf("%s... %d descendants (use --depth %d to expand)", r.Indent, r.Hidden, r.Expand)
	}
	if r.Located {
		return r.Indent + r.Class + "  (" + r.Module + ")"
	}
	return r.Indent + r.Class
}

// Hierarchy is the subclass tree of Root.
type Hierarchy struct {
	Root        string
	Children    int // direct subclasses
	Descendants int // subclasses at every level, each counted once
	Module      string
	Package     string
	Located     bool      // Module and Package are known
	Rows        []TreeRow // the root first, then depth first in index order
}

// Subclasses is the subclass tree of name down to maxDepth levels (`hierarchy
// <name> --depth maxDepth`); ok is false when name has no subclasses.
func Subclasses(inh, classes *index.Object, name string, maxDepth int) (h Hierarchy, ok bool) {
	p2c := TableOf(inh, "parent_to_children")
	if !p2c.Has(name) {
		return Hierarchy{}, false
	}
	h = Hierarchy{Root: name, Children: len(p2c.List(name)),
		Descendants: countDescendants(name, p2c, map[string]bool{})}
	h.Module, h.Package, h.Located = moduleInfo(name, classes)
	h.Rows = treeRows(nil, name, p2c, classes, maxDepth, "  ", true, 0)
	return h, true
}

// treeRows is _print_tree: one class per row under its parent, cut at
// maxDepth with a descendant count.
func treeRows(rows []TreeRow, name string, p2c Table, classes *index.Object, maxDepth int,
	prefix string, isLast bool, depth int) []TreeRow {
	connector, childPrefix := "", prefix
	if depth > 0 {
		connector, childPrefix = "|-- ", prefix+"|   "
		if isLast {
			connector, childPrefix = "`-- ", prefix+"    "
		}
	}
	mod, _, located := moduleInfo(name, classes)
	rows = append(rows, TreeRow{Class: name, Module: mod, Located: located, Depth: depth, Indent: prefix + connector})
	children := p2c.List(name)
	next := childPrefix
	if depth == 0 {
		next = prefix + "    "
	}
	if depth >= maxDepth {
		if len(children) > 0 {
			rows = append(rows, TreeRow{Depth: depth + 1, Indent: next,
				Hidden: countDescendants(name, p2c, map[string]bool{}), Expand: maxDepth + 1})
		}
		return rows
	}
	for i, c := range children {
		rows = treeRows(rows, c, p2c, classes, maxDepth, next, i == len(children)-1, depth+1)
	}
	return rows
}

// Chain is the inheritance chain of name (`hierarchy <name> --chain`): name,
// then its superclasses nearest first; ok is false when inheritance.json has
// no chain for it (a root class, an interface or an unknown name).
func Chain(inh, classes *index.Object, name string) (rows []ClassRow, ok bool) {
	c2c := TableOf(inh, "class_to_chain")
	if !c2c.Has(name) {
		return nil, false
	}
	for _, c := range append([]string{name}, c2c.List(name)...) {
		rows = append(rows, classRow(c, classes))
	}
	return rows, true
}

// FormatChain is the chain lines `hierarchy --chain` prints, one per row:
// each superclass indented one more step under "^-- ", the class itself
// right-justified to the longest name.
func FormatChain(rows []ClassRow) []string {
	width := 0
	for _, r := range rows {
		width = max(width, len([]rune(r.Class)))
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		arrow, label := "", r.Class
		if i > 0 {
			arrow = "^-- "
		} else {
			label = core.RJust(r.Class, width)
		}
		out[i] = fmt.Sprintf("  %s%s%s  (%s, %s)", strings.Repeat("  ", i), arrow, label, r.Module, r.Package)
	}
	return out
}

// Implementors is the classes implementing iface (`implementors <iface>`), in
// index order, with iface resolved ignoring case when it is not a key as
// given; ok is false when no interface of that name has implementors.
func Implementors(inh, classes *index.Object, iface string) (name string, rows []ClassRow, ok bool) {
	i2i := TableOf(inh, "interface_to_implementors")
	name, ok = iface, i2i.Has(iface)
	if !ok {
		lower := textfold.Lower(iface)
		for _, k := range i2i.Keys() {
			if textfold.Lower(k) == lower {
				name, ok = k, true
				break
			}
		}
	}
	if !ok {
		return iface, nil, false
	}
	for _, c := range i2i.List(name) {
		rows = append(rows, classRow(c, classes))
	}
	return name, rows, true
}

// FormatImplementor is the "CLASS  MODULE  PACKAGE" row of `implementors`:
// the class right-justified to 35, the module left-justified to 20.
func FormatImplementor(r ClassRow) string {
	return fmt.Sprintf("  %s  %s  %s", core.RJust(r.Class, 35), core.LJust(r.Module, 20), r.Package)
}
