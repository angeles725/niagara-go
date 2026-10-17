package help

// Inheritance tree navigation over inheritance.json (hierarchy.py):
// hierarchy (subclass tree or --chain) and implementors.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

var htmlGenerics = regexp.MustCompile(`&lt;.*?&gt;`)

// stripGenerics removes HTML-encoded generics such as &lt;E&gt; from a key.
func stripGenerics(name string) string { return htmlGenerics.ReplaceAllString(name, "") }

// resolveInheritanceKey is hierarchy.resolve_inheritance_key: the key of a
// simple, generic-stripped, qualified or case-insensitive name ("" if none).
// Each map keeps the first key that produces its name (dict.setdefault).
func resolveInheritanceKey(inh *index.Object, name string) string {
	if inh.Has(name) {
		return name
	}
	var stripped, qual, lower string
	for _, key := range inh.Keys() {
		s := stripGenerics(key)
		if stripped == "" && s == name {
			stripped = key
		}
		if lower == "" && lowerStr(s) == lowerStr(name) {
			lower = key
		}
		if pv := get(inh.Object(key), "package", nil); qual == "" && truthy(pv) {
			pkg, simple := pyStr(pv), s
			if strings.HasPrefix(s, pkg+".") {
				simple = s[len(pkg)+1:]
			}
			if pkg+"."+simple == name {
				qual = key
			}
		}
	}
	switch {
	case stripped != "":
		return stripped
	case qual != "":
		return qual
	}
	return lower
}

// bareType is _bare: lower-case type name without generics, "$" as ".".
func bareType(t string) string {
	s := stripGenerics(t)
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[:i]
	}
	return lowerStr(strings.ReplaceAll(pycompat.Strip(s), "$", "."))
}

type implementor struct{ name, pkg string }

// implementorMatches is hierarchy.implementor_matches: the classes (key
// without generics, package) implementing iface. A simple name matches every
// interface with that simple name, a qualified one only that interface.
func implementorMatches(inh *index.Object, iface string) []implementor {
	want := bareType(iface)
	wantSimple := want[strings.LastIndex(want, ".")+1:]
	qualified := strings.Contains(want, ".")
	var out []implementor
	for _, key := range inh.Keys() {
		entry := inh.Object(key)
		for _, impl := range strList(entry, "implements") {
			got := bareType(impl)
			var same bool
			if qualified && strings.Contains(got, ".") {
				same = got == want
			} else {
				same = got[strings.LastIndex(got, ".")+1:] == wantSimple
			}
			if same {
				pkg := getStr(entry, "package", "")
				name := stripGenerics(key)
				if pkg != "" && strings.HasPrefix(name, pkg+".") {
					name = name[len(pkg)+1:] // FQN key of a collision
				}
				out = append(out, implementor{name, pkg})
				break
			}
		}
	}
	return out
}

// buildLookup maps every key and its generic-stripped form to the key; a
// later key wins a shared stripped form (_build_lookup).
func buildLookup(inh *index.Object) map[string]string {
	lookup := map[string]string{}
	for _, key := range inh.Keys() {
		lookup[stripGenerics(key)] = key
		lookup[key] = key
	}
	return lookup
}

// subclassesOf is inheritance[lookup[name]]["subclasses"], ok false when
// the name has no entry.
func subclassesOf(inh *index.Object, lookup map[string]string, name string) ([]string, bool) {
	key, ok := lookup[name]
	if !ok || !inh.Has(key) {
		return nil, false
	}
	return strList(inh.Object(key), "subclasses"), true
}

func (e *Env) printTree(inh *index.Object, lookup map[string]string, name string, depth, maxDepth int, prefix string) {
	subs, ok := subclassesOf(inh, lookup, name)
	if !ok || len(subs) == 0 {
		return
	}
	sorted := append([]string(nil), subs...)
	sort.Strings(sorted)
	for i, sub := range sorted {
		last := i == len(sorted)-1
		connector, childPrefix := "|-- ", "|   "
		if last {
			connector, childPrefix = "`-- ", "    "
		}
		grand, _ := subclassesOf(inh, lookup, sub)
		suffix := ""
		if len(grand) > 0 && depth >= maxDepth {
			suffix = " (+)"
		}
		fmt.Fprintf(e.Out, "%s%s%s%s\n", prefix, connector, sub, suffix)
		if depth < maxDepth {
			e.printTree(inh, lookup, sub, depth+1, maxDepth, prefix+childPrefix)
		}
	}
}

// inheritanceChain is _get_chain: name up to the root, root first; a
// java.lang parent ends the chain.
func inheritanceChain(inh *index.Object, lookup map[string]string, name string) []string {
	chain := []string{name}
	seen := map[string]bool{}
	for cur := name; cur != "" && !seen[cur]; {
		seen[cur] = true
		key, ok := lookup[cur]
		if !ok || !inh.Has(key) {
			break
		}
		parent := getStr(inh.Object(key), "extends", "")
		if parent == "" || strings.HasPrefix(parent, "java.lang.") {
			if parent != "" {
				chain = append(chain, parent)
			}
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// HierarchyParams are the arguments of `hierarchy`.
type HierarchyParams struct {
	Name      string
	Depth     *int // nil: 2, or 3 with Interface
	Chain     bool
	Interface bool // interface view (interface-hierarchy)
}

// Hierarchy shows the subclass tree of a class, its inheritance chain, or
// with Interface the interface hierarchy (cmd_hierarchy).
func (e *Env) Hierarchy(p HierarchyParams) error {
	if p.Interface {
		return notPortedError("hierarchy --interface")
	}
	maxDepth := 2
	if p.Depth != nil {
		maxDepth = *p.Depth
	}
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	lookup := buildLookup(inh)
	name := p.Name
	if _, ok := lookup[name]; !ok {
		key := resolveInheritanceKey(inh, name)
		if key == "" {
			return e.reportMiss("hierarchy", name, "inheritance")
		}
		name = key
		if _, ok := lookup[key]; !ok {
			lookup[key] = key
		}
	}
	entry := inh.Object(lookup[name])
	impl := strList(entry, "implements")
	if p.Chain {
		fmt.Fprintf(e.Out, "Inheritance chain for %s:\n", name)
		fmt.Fprintf(e.Out, "  %s\n", strings.Join(inheritanceChain(inh, lookup, name), " > "))
		if len(impl) > 0 {
			fmt.Fprintf(e.Out, "  implements: %s\n", strings.Join(impl, ", "))
		}
		return nil
	}
	subCount := listLen(entry, "subclasses")
	fmt.Fprintf(e.Out, "%s (%s)\n", name, getStr(entry, "package", "?"))
	fmt.Fprintf(e.Out, "  extends: %s\n", getStr(entry, "extends", "?"))
	if len(impl) > 0 {
		fmt.Fprintf(e.Out, "  implements: %s\n", strings.Join(impl, ", "))
	}
	fmt.Fprintf(e.Out, "  direct subclasses: %d\n", subCount)
	if subCount > 0 {
		fmt.Fprintf(e.Out, "\n%s\n", name)
		e.printTree(inh, lookup, name, 1, maxDepth, "")
	}
	return nil
}

// ImplementorsParams are the arguments of `implementors`.
type ImplementorsParams struct{ Name string }

// Implementors lists the classes implementing an interface (cmd_implementors).
func (e *Env) Implementors(p ImplementorsParams) error {
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	results := implementorMatches(inh, p.Name)
	if len(results) == 0 {
		if key, ok := buildLookup(inh)[p.Name]; ok && truthy(get(inh.Object(key), "subclasses", nil)) {
			fmt.Fprintf(e.Out, "'%s' is a class, not an interface. Use 'hierarchy' instead.\n", p.Name)
			return nil
		}
		return e.reportMiss("implementors", p.Name, "inheritance")
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].name < results[j].name })
	fmt.Fprintf(e.Out, "%d classes implement '%s':\n\n", len(results), p.Name)
	for _, r := range results {
		fmt.Fprintf(e.Out, "  %s (%s)\n", r.name, r.pkg)
	}
	return nil
}
