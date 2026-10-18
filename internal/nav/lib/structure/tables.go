package structure

import (
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// The inheritance.json table helpers and module lookup Subclasses, Chain and
// Implementors share with the hierarchy commands (internal/nav/structure).

const docSource = "docSource-doc"

// Table is a {name: [names]} table of inheritance.json (nil-safe lookups).
type Table struct{ o *index.Object }

// Has is k in t.
func (t Table) Has(k string) bool { return t.o != nil && t.o.Has(k) }

// List is t.get(k, []) as strings.
func (t Table) List(k string) []string {
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

// Len is len(t).
func (t Table) Len() int {
	if t.o == nil {
		return 0
	}
	return t.o.Len()
}

// Keys is list(t), in index order.
func (t Table) Keys() []string {
	if t.o == nil {
		return nil
	}
	return t.o.Keys()
}

// TableOf is inh.get(k, {}).
func TableOf(o *index.Object, k string) Table { return Table{o.Object(k)} }

// TopEntries are the entries of a class without outer_class.
func TopEntries(classes *index.Object, name string) []*index.Object {
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
	top := TopEntries(classes, name)
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
func countDescendants(name string, p2c Table, seen map[string]bool) int {
	if seen[name] {
		return 0
	}
	seen[name] = true
	children := p2c.List(name)
	n := len(children)
	for _, c := range children {
		n += countDescendants(c, p2c, seen)
	}
	return n
}
