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
	"github.com/angeles725/niagara-go/internal/textfold"
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
	p2c := tableOf(inh, "parent_to_children")
	classes := classTable(env)
	w := env.Out
	if !p2c.has(name) {
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
	total := countDescendants(name, p2c, map[string]bool{})
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Hierarchy: %s (%d direct children, %d total descendants)\n", name,
		len(p2c.list(name)), total)
	if mod, pkg, ok := moduleInfo(name, classes); ok {
		fmt.Fprintf(w, "  Module: %s  Package: %s\n", mod, pkg)
	}
	fmt.Fprintf(w, "  Depth limit: %d\n", maxDepth)
	fmt.Fprintln(w)
	printTree(w, name, p2c, classes, maxDepth, "  ", true, 0)
	fmt.Fprintln(w)
}

// printTree is _print_tree: one class per line under its parent, cut at
// maxDepth with a descendant count.
func printTree(w io.Writer, name string, p2c table, classes *index.Object, maxDepth int,
	prefix string, isLast bool, depth int) {
	connector, childPrefix := "", prefix
	if depth > 0 {
		connector, childPrefix = "|-- ", prefix+"|   "
		if isLast {
			connector, childPrefix = "`-- ", prefix+"    "
		}
	}
	moduleStr := ""
	if mod, _, ok := moduleInfo(name, classes); ok {
		moduleStr = "  (" + mod + ")"
	}
	fmt.Fprintf(w, "%s%s%s%s\n", prefix, connector, name, moduleStr)
	children := p2c.list(name)
	next := childPrefix
	if depth == 0 {
		next = prefix + "    "
	}
	if depth >= maxDepth {
		if len(children) > 0 {
			fmt.Fprintf(w, "%s... %d descendants (use --depth %d to expand)\n", next,
				countDescendants(name, p2c, map[string]bool{}), maxDepth+1)
		}
		return
	}
	for i, c := range children {
		printTree(w, c, p2c, classes, maxDepth, next, i == len(children)-1, depth+1)
	}
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
	c2c := tableOf(inh, "class_to_chain")
	classes := classTable(env)
	w := env.Out
	known := classes != nil && classes.Len() > 0 && classes.Has(name)
	if !c2c.has(name) {
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
	chain := c2c.list(name)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Inheritance chain for %s (depth %d):\n", name, len(chain))
	fmt.Fprintln(w)
	all := append([]string{name}, chain...)
	width := 0
	for _, c := range all {
		width = max(width, len([]rune(c)))
	}
	for i, c := range all {
		mod, pkg, ok := moduleInfo(c, classes)
		if !ok {
			mod, pkg = "?", "?"
		}
		arrow, label := "", c
		if i > 0 {
			arrow = "^-- "
		} else {
			label = core.RJust(c, width)
		}
		fmt.Fprintf(w, "  %s%s%s  (%s, %s)\n", strings.Repeat("  ", i), arrow, label, mod, pkg)
	}
	if known {
		printImplements(w, classes, name, true)
	}
	if len(chain) > 0 {
		parent := chain[0]
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
	i2i := tableOf(inh, "interface_to_implementors")
	classes := classTable(env)
	if !i2i.has(name) {
		lower, found := textfold.Lower(name), false
		for _, k := range i2i.keys() {
			if textfold.Lower(k) == lower {
				name, found = k, true
				break
			}
		}
		if !found {
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
	}
	impls := i2i.list(name)
	w := env.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Implementors of %s: %d classes\n", name, len(impls))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s  %s\n", core.RJust("CLASS", 35), core.LJust("MODULE", 20), "PACKAGE")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 85))
	for i, c := range impls {
		if i >= limit {
			break
		}
		mod, pkg, ok := moduleInfo(c, classes)
		if !ok {
			mod, pkg = "?", "?"
		}
		fmt.Fprintf(w, "  %s  %s  %s\n", core.RJust(c, 35), core.LJust(mod, 20), pkg)
	}
	if len(impls) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %d more (use -n to show more)\n", len(impls)-limit)
	}
	fmt.Fprintln(w)
}
