package structure

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// The hierarchy commands of hierarchy.py: hierarchy (subclass tree or
// inheritance chain) and implementors, over inheritance.json with module and
// package from the class index.

const docSource = "docSource-doc"

// table is a {name: [names]} table of inheritance.json (nil-safe lookups).
type table struct{ o *index.Object }

func (t table) has(k string) bool { return t.o != nil && t.o.Has(k) }

// list is t.get(k, []) as strings.
func (t table) list(k string) []string {
	if t.o == nil {
		return nil
	}
	v, _ := t.o.Get(k)
	l, _ := v.([]any)
	out := make([]string, len(l))
	for i, x := range l {
		out[i] = core.PyStr(x)
	}
	return out
}

func (t table) len() int {
	if t.o == nil {
		return 0
	}
	return t.o.Len()
}

func (t table) keys() []string {
	if t.o == nil {
		return nil
	}
	return t.o.Keys()
}

// tableOf is inh.get(k, {}).
func tableOf(o *index.Object, k string) table { return table{o.Object(k)} }

// loadInheritance is load_inheritance plus `if not inh: return`.
func loadInheritance(env *registry.Env) *index.Object {
	inh := env.Load("inheritance.json", core.LoadOpts{})
	if inh == nil || inh.Len() == 0 {
		return nil
	}
	return inh
}

// classTable is `ci_data.get("classes", {}) if ci_data else {}`.
func classTable(env *registry.Env) *index.Object {
	if ci := env.LoadClassIndex(); ci != nil {
		return ci.Object("classes")
	}
	return nil
}

// topEntries are the entries of a class without outer_class.
func topEntries(classes *index.Object, name string) []*index.Object {
	if classes == nil {
		return nil
	}
	var out []*index.Object
	for _, e := range core.Entries(classes, name) {
		if !core.Truthy(e, "outer_class") {
			out = append(out, e)
		}
	}
	return out
}

// moduleInfo is _get_module_info: (module, package) of a class's top-level
// entry; a runtime copy wins over the docSource-doc copy (an -rt module
// first), noted as "(+doc source)".
func moduleInfo(name string, classes *index.Object) (module, pkg string, ok bool) {
	top := topEntries(classes, name)
	if len(top) == 0 {
		return "", "", false
	}
	var runtime []*index.Object
	for _, e := range top {
		if core.Str(e, "module") != docSource {
			runtime = append(runtime, e)
		}
	}
	sort.SliceStable(runtime, func(a, b int) bool {
		return strings.HasSuffix(core.Str(runtime[a], "module"), "-rt") &&
			!strings.HasSuffix(core.Str(runtime[b], "module"), "-rt")
	})
	if len(runtime) == 0 {
		return core.Str(top[0], "module"), core.Str(top[0], "package"), true
	}
	doc := ""
	if len(runtime) < len(top) {
		doc = " (+doc source)"
	}
	return core.Str(runtime[0], "module") + doc, core.Str(runtime[0], "package"), true
}

// countDescendants is _count_descendants: children counted at every level,
// each class descended into once.
func countDescendants(name string, p2c table, seen map[string]bool) int {
	if seen[name] {
		return 0
	}
	seen[name] = true
	children := p2c.list(name)
	n := len(children)
	for _, c := range children {
		n += countDescendants(c, p2c, seen)
	}
	return n
}

func runHierarchy(env *registry.Env, ns *argparse.Namespace) int {
	if ns.Bool("chain") {
		showChain(env, ns.String("name"))
	} else {
		showSubtree(env, ns.String("name"), ns.Int("depth"))
	}
	return env.ExitCode()
}

func showSubtree(env *registry.Env, name string, maxDepth int) {
	inh := loadInheritance(env)
	if inh == nil {
		return
	}
	classes := classTable(env)
	w := env.Out
	h, ok := Subclasses(inh, classes, name, maxDepth)
	if !ok {
		if classes != nil && classes.Len() > 0 && classes.Has(name) {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  %s has no known subclasses.\n", name)
			if c2c := tableOf(inh, "class_to_chain"); c2c.has(name) {
				fmt.Fprintf(w, "  Chain: %s -> %s\n", name, strings.Join(c2c.list(name), " -> "))
			}
			fmt.Fprintln(w)
		} else {
			env.ClassMiss(name, nil, "", fmt.Sprintf("search '*%s*'", name))
		}
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Hierarchy: %s (%d direct children, %d total descendants)\n", name,
		h.Children, h.Descendants)
	if h.Located {
		fmt.Fprintf(w, "  Module: %s  Package: %s\n", h.Module, h.Package)
	}
	fmt.Fprintf(w, "  Depth limit: %d\n", maxDepth)
	fmt.Fprintln(w)
	for _, r := range h.Rows {
		fmt.Fprintln(w, FormatTreeRow(r))
	}
	fmt.Fprintln(w)
}

// printImplements prints "  Implements: ..." when the class's first top-level
// entry implements interfaces (blank says whether a blank line goes first).
func printImplements(w io.Writer, classes *index.Object, name string, blank bool) {
	top := topEntries(classes, name)
	if len(top) > 0 && core.Truthy(top[0], "implements") {
		if blank {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "  Implements: %s\n", strings.Join(core.Strs(top[0], "implements"), ", "))
	}
}

func showChain(env *registry.Env, name string) {
	inh := loadInheritance(env)
	if inh == nil {
		return
	}
	classes := classTable(env)
	w := env.Out
	known := classes != nil && classes.Len() > 0 && classes.Has(name)
	rows, ok := Chain(inh, classes, name)
	if !ok {
		if known {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  %s has no superclass chain (root class or interface).\n", name)
			printImplements(w, classes, name, false)
			fmt.Fprintln(w)
		} else {
			env.ClassMiss(name, nil, "", fmt.Sprintf("search '*%s*'", name))
		}
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Inheritance chain for %s (depth %d):\n", name, len(rows)-1)
	fmt.Fprintln(w)
	for _, line := range FormatChain(rows) {
		fmt.Fprintln(w, line)
	}
	if known {
		printImplements(w, classes, name, true)
	}
	if len(rows) > 1 {
		parent := rows[1].Class
		if n := len(tableOf(inh, "parent_to_children").list(parent)) - 1; n > 0 {
			fmt.Fprintf(w, "  Siblings under %s: %d\n", parent, n)
		}
	}
	fmt.Fprintln(w)
}

func runImplementors(env *registry.Env, ns *argparse.Namespace) int {
	cmdImplementors(env, ns.String("name"), ns.Int("n"))
	return env.ExitCode()
}

func cmdImplementors(env *registry.Env, name string, limit int) {
	inh := loadInheritance(env)
	if inh == nil {
		return
	}
	classes := classTable(env)
	name, impls, ok := Implementors(inh, classes, name)
	if !ok {
		i2i := tableOf(inh, "interface_to_implementors")
		scope := fmt.Sprintf("%s interfaces with implementors in inheritance.json, %s",
			core.Commas(int64(i2i.len())), core.Built(inh.Object("_meta")))
		if tableOf(inh, "parent_to_children").has(name) {
			env.Miss(fmt.Sprintf("implementors of '%s' (a class with subclasses, not an interface)", name),
				scope, "hierarchy "+name)
		} else {
			env.Report(fmt.Sprintf("interface '%s'", name), scope, name, i2i.keys(),
				fmt.Sprintf("search '*%s*'", name))
		}
		return
	}
	w := env.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Implementors of %s: %d classes\n", name, len(impls))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s  %s\n", core.RJust("CLASS", 35), core.LJust("MODULE", 20), "PACKAGE")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 85))
	for _, r := range impls[:min(max(limit, 0), len(impls))] {
		fmt.Fprintln(w, FormatImplementor(r))
	}
	if len(impls) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %d more (use -n to show more)\n", len(impls)-limit)
	}
	fmt.Fprintln(w)
}
