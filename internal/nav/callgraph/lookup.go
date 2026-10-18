package callgraph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The lookups of cmd_callers and cmd_callees, and the lines they print.

// Callers is `callers <Class> <method>` (cmd_callers with a method).
type Callers struct {
	Class, Method string      // as resolved (a case-insensitive key renames them)
	Key           string      // Class.method
	TargetModule  string      // module of Class
	Rows          []CallerRow // every caller, sorted; empty on a miss
	Partial       []Partial   // on a miss: other classes with callers for Method, sorted
}

// CallerRow is one caller of a method.
type CallerRow struct{ Caller, Module, Class string }

// Partial is one "Other classes with callers" key and its caller count.
type Partial struct {
	Key     string
	Callers int
}

// CallersOf is cmd_callers for one method: key Class.method as given, else
// the reverse-index key equal to it ignoring case.
func (g *Graph) CallersOf(classes *index.Object, class, method string) Callers {
	key := class + "." + method
	callers := g.CalledBy(key)
	if len(callers) == 0 { // not a key: the one equal to it ignoring case, if any
		key = g.AnyCase(key)
		class, method = classPart(key), methodPart(key)
		callers = g.CalledBy(key)
	}
	r := Callers{Class: class, Method: method, Key: key}
	if len(callers) == 0 {
		suffix := "." + method
		for _, c := range g.collect(func(k string) bool { return strings.HasSuffix(k, suffix) }) {
			r.Partial = append(r.Partial, Partial{c.key, len(c.callers)})
		}
		slices.SortFunc(r.Partial, func(a, b Partial) int { return strings.Compare(a.Key, b.Key) })
		return r
	}
	r.TargetModule = ModuleOf(classes, class)
	for _, c := range callers {
		cls := c
		if dot := strings.LastIndexByte(c, '.'); dot > 0 {
			cls = c[:dot]
		}
		r.Rows = append(r.Rows, CallerRow{c, ModuleOf(classes, cls), cls})
	}
	return r
}

// CallersHeader and CallersRule head the caller table of `callers`.
var (
	CallersHeader = fmt.Sprintf("  %s  %s  %s", core.RJust("CALLER METHOD", 40), core.LJust("MODULE", 25), "CLASS")
	CallersRule   = "  " + strings.Repeat("-", 90)
)

// Line is the caller table line of r.
func (r CallerRow) Line() string {
	return fmt.Sprintf("  %s  %s  %s", core.RJust(r.Caller, 40), core.LJust(r.Module, 25), r.Class)
}

// Line is the "Other classes with callers" line of p.
func (p Partial) Line() string { return fmt.Sprintf("    %s (%d callers)", p.Key, p.Callers) }

// ClassCallers is `callers <Class>` (_callers_of_class): the callers of every
// method of the class.
type ClassCallers struct {
	Class        string      // as resolved (case-insensitive when no exact match)
	Ambiguous    []string    // several classes match ignoring case: sorted, nothing else is set
	TargetModule string      // module of Class
	Methods      []MethodRow // called methods, most callers first
	Total        int         // call sites (sum of the method caller counts)
	Sites        []SiteRow   // every (caller, method), in Methods order
}

// MethodRow is one called method of the class with its caller count.
type MethodRow struct {
	Method  string
	Callers int
}

// SiteRow is one call site of `callers <Class>`.
type SiteRow struct{ Caller, Module, Method string }

// CallersOfClass is _callers_of_class: the exact class segment first ("Outer.m",
// never "Outer.Inner.m" nor "OuterExt.m"), else the class ignoring case. A
// miss leaves Methods empty.
func (g *Graph) CallersOfClass(classes *index.Object, class string) ClassCallers {
	keys := g.collect(func(k string) bool { return classPart(k) == class })
	if len(keys) == 0 {
		lower := textfold.Lower(class)
		keys = g.collect(func(k string) bool { return sameLower(classPart(k), class, lower) })
		var names []string
		for _, k := range keys {
			names = append(names, classPart(k.key))
		}
		slices.Sort(names)
		if names = slices.Compact(names); len(names) > 1 {
			return ClassCallers{Class: class, Ambiguous: names}
		}
	}
	r := ClassCallers{Class: class}
	if len(keys) == 0 {
		return r
	}
	r.Class = classPart(keys[0].key)
	slices.SortFunc(keys, func(a, b counted) int {
		if len(a.callers) != len(b.callers) {
			return len(b.callers) - len(a.callers)
		}
		return strings.Compare(a.key, b.key)
	})
	r.TargetModule = ModuleOf(classes, r.Class)
	for _, k := range keys {
		m := methodPart(k.key)
		r.Methods = append(r.Methods, MethodRow{m, len(k.callers)})
		r.Total += len(k.callers)
		for _, c := range k.callers {
			cls := classPart(c)
			if cls == "" {
				cls = c
			}
			r.Sites = append(r.Sites, SiteRow{c, ModuleOf(classes, cls), m})
		}
	}
	return r
}

// MethodsHeader and MethodsRule head the method table of `callers <Class>`;
// SitesHeader and SitesRule its call-site table.
var (
	MethodsHeader = fmt.Sprintf("  %s  %s", core.RJust("METHOD", 40), core.RJust("CALLERS", 7))
	MethodsRule   = "  " + strings.Repeat("-", 50)
	SitesHeader   = fmt.Sprintf("  %s  %s  %s", core.RJust("CALLER METHOD", 40), core.LJust("MODULE", 25), "CALLS")
	SitesRule     = "  " + strings.Repeat("-", 90)
)

// Line is the method table line of m.
func (m MethodRow) Line() string {
	return fmt.Sprintf("  %s  %s", core.RJust(m.Method, 40), core.RJust(core.Commas(int64(m.Callers)), 7))
}

// Line is the call-site table line of s.
func (s SiteRow) Line() string {
	return fmt.Sprintf("  %s  %s  %s", core.RJust(s.Caller, 40), core.LJust(s.Module, 25), s.Method)
}

// CalleeRow is one called method with the module of its class.
type CalleeRow struct{ Callee, Module string }

// calleeRow is the row of callee: its class is the name before the last '.'.
func calleeRow(classes *index.Object, callee string) CalleeRow {
	cls := callee
	if dot := strings.LastIndexByte(callee, '.'); dot > 0 {
		cls = callee[:dot]
	}
	return CalleeRow{callee, ModuleOf(classes, cls)}
}

// Callees is `callees <Class> <method>`: the calls of Class.method in index
// order; on a miss, Rows is empty and ClassMethods lists the class's methods
// with calls (sorted keys).
type Callees struct {
	Key          string
	Rows         []CalleeRow
	ClassMethods []string
}

// CalleesOf is cmd_callees with a method.
func (g *Graph) CalleesOf(classes *index.Object, class, method string) Callees {
	key := class + "." + method
	r := Callees{Key: key}
	for i, caller := range g.callers {
		if caller == key {
			for _, c := range g.callees[i] {
				r.Rows = append(r.Rows, calleeRow(classes, c))
			}
		}
	}
	if len(r.Rows) > 0 {
		return r
	}
	prefix := class + "."
	for _, caller := range g.callers {
		if strings.HasPrefix(caller, prefix) {
			r.ClassMethods = append(r.ClassMethods, caller)
		}
	}
	slices.Sort(r.ClassMethods)
	return r
}

// CalleesHeader and CalleesRule head the callee table of `callees <Class> <method>`.
var (
	CalleesHeader = fmt.Sprintf("  %s  %s", core.RJust("CALLED METHOD", 40), core.LJust("MODULE", 25))
	CalleesRule   = "  " + strings.Repeat("-", 65)
)

// Line is the callee table line of r.
func (r CalleeRow) Line() string {
	return fmt.Sprintf("  %s  %s", core.RJust(r.Callee, 40), core.LJust(r.Module, 25))
}

// ArrowLine is the `callees <Class>` line of r under its method.
func (r CalleeRow) ArrowLine() string {
	return fmt.Sprintf("    -> %s  [%s]", core.LJust(r.Callee, 40), r.Module)
}

// MethodCallees is one method of `callees <Class>` with its calls.
type MethodCallees struct {
	Key, Method string // Method is the key after its first '.'
	Calls       int
	Rows        []CalleeRow // the first 10 calls
}

// ClassCallees is `callees <Class>`: the methods of the class with calls
// (sorted keys), or on a miss the class names that contain Class ignoring case.
type ClassCallees struct {
	Methods    []MethodCallees
	Total      int // calls of every method
	Module     string
	DidYouMean []string // on a miss: up to 10 sorted class names
}

// CalleesOfClass is cmd_callees without a method.
func (g *Graph) CalleesOfClass(classes *index.Object, class string) ClassCallees {
	prefix := class + "."
	var r ClassCallees
	type entry struct {
		key   string
		calls []string
	}
	var entries []entry // callers are unique keys of the index
	for i, caller := range g.callers {
		if strings.HasPrefix(caller, prefix) {
			entries = append(entries, entry{caller, g.callees[i]})
		}
	}
	if len(entries) == 0 {
		lower := textfold.Lower(class)
		var names []string
		for _, caller := range g.callers {
			head := caller
			if k := strings.IndexByte(caller, '.'); k >= 0 {
				head = caller[:k]
			}
			if strings.Contains(textfold.Lower(head), lower) {
				names = append(names, head)
			}
		}
		slices.Sort(names)
		names = slices.Compact(names)
		r.DidYouMean = names[:min(10, len(names))]
		return r
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.key, b.key) })
	r.Module = ModuleOf(classes, class)
	for _, e := range entries {
		r.Total += len(e.calls)
		m := e.key
		if k := strings.IndexByte(e.key, '.'); k >= 0 {
			m = e.key[k+1:]
		}
		mc := MethodCallees{Key: e.key, Method: m, Calls: len(e.calls)}
		for _, c := range e.calls[:min(10, len(e.calls))] {
			mc.Rows = append(mc.Rows, calleeRow(classes, c))
		}
		r.Methods = append(r.Methods, mc)
	}
	return r
}
