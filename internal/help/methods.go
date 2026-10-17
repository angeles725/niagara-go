package help

// Method views over the inheritance chain and method-index.json:
// inherited-methods (method_search.cmd_inherited_methods) and overrides
// (overrides_search.cmd_overrides).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// classMethods maps every class of chain to its methods (name -> signature):
// method-index entries of the "methods" section, the last entry of a name
// winning for its class, as the dict assignments of the Python commands do.
func classMethods(methods *index.Object, chain []string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(chain))
	for _, c := range chain {
		out[c] = map[string]string{}
	}
	for _, name := range methods.Keys() {
		entries, _ := get(methods, name, nil).([]any)
		for _, v := range entries {
			entry, _ := v.(*index.Object)
			cls, _ := get(entry, "class", "").(string)
			if m, ok := out[cls]; ok && pyStr(get(entry, "section", nil)) == "methods" {
				m[name] = getStr(entry, "signature", "")
			}
		}
	}
	return out
}

// sortedKeys returns the keys of m in sorted() order.
func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// origin is where a method of the chain is declared and its signature there.
type origin struct{ class, sig string }

// parentMethods is the parent_methods dict of api and overrides: every
// method of parents (self-first order) mapped to its nearest declaration.
func parentMethods(cm map[string]map[string]string, parents []string) map[string]origin {
	out := map[string]origin{}
	for i := len(parents) - 1; i >= 0; i-- {
		for name, sig := range cm[parents[i]] {
			out[name] = origin{parents[i], sig}
		}
	}
	return out
}

// InheritedMethodsParams are the arguments of `inherited-methods`.
type InheritedMethodsParams struct{ Name string }

// InheritedMethods is method_search.cmd_inherited_methods: every method of
// the chain, under the class that declares its effective version.
func (e *Env) InheritedMethods(p InheritedMethodsParams) error {
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	methods, err := e.loadIndex("method-index.json", false)
	if err != nil {
		return err
	}
	name := p.Name
	if !inh.Has(name) {
		s, ok := resolveStripped(inh, name)
		if !ok {
			return e.reportMiss("inherited-methods", name, "inheritance")
		}
		name = s
	}
	chain := reversed(ancestry(inh, name))
	cm := classMethods(methods, chain)
	effective := map[string]origin{}
	for _, c := range chain {
		for m, sig := range cm[c] {
			effective[m] = origin{c, sig}
		}
	}
	byOrigin := map[string][]string{}
	for _, m := range sortedKeys(effective) {
		o := effective[m]
		byOrigin[o.class] = append(byOrigin[o.class], o.sig)
	}
	fmt.Fprintf(e.Out, "INHERITED METHODS: %s\n", name)
	fmt.Fprintf(e.Out, "CHAIN: %s\n", strings.Join(chain, " > "))
	fmt.Fprintf(e.Out, "TOTAL: %d methods\n", len(effective))
	for _, c := range chain {
		sigs := byOrigin[c]
		if len(sigs) == 0 {
			continue
		}
		marker := ""
		if c == name {
			marker = " (this)"
		}
		fmt.Fprintf(e.Out, "\n  -- from %s%s (%d) --\n", c, marker, len(sigs))
		for _, sig := range sigs {
			fmt.Fprintf(e.Out, "    %s\n", sig)
		}
	}
	return nil
}

// resolveClassKey is the class resolution of overrides and contract: the
// exact class-index key, else the first key equal case-insensitively.
func resolveClassKey(idx *index.Object, name string) (string, bool) {
	if idx.Has(name) {
		return name, true
	}
	want := lowerStr(name)
	for _, k := range idx.Keys() {
		if lowerStr(k) == want {
			return k, true
		}
	}
	return "", false
}

// OverridesParams are the arguments of `overrides`.
type OverridesParams struct{ Name string }

// Overrides is overrides_search.cmd_overrides: the methods a class
// overrides, adds and inherits unchanged.
func (e *Env) Overrides(p OverridesParams) error {
	classes, err := e.loadIndex("class-index.json", true)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", true)
	if err != nil {
		return err
	}
	methods, err := e.loadIndex("method-index.json", true)
	if err != nil {
		return err
	}
	name, ok := resolveClassKey(classes, p.Name)
	if !ok {
		return e.reportMiss("overrides", p.Name, "classes")
	}
	chain := ancestry(inh, name)
	if len(chain) < 2 {
		fmt.Fprintf(e.Out, "'%s' has no parent class in the index.\n", name)
		return nil
	}
	cm := classMethods(methods, chain)
	mine := cm[name]
	parents := parentMethods(cm, chain[1:])
	var overridden, added, inherited []string
	for _, m := range sortedKeys(mine) {
		if _, ok := parents[m]; ok {
			overridden = append(overridden, m)
		} else {
			added = append(added, m)
		}
	}
	for _, m := range sortedKeys(parents) {
		if _, ok := mine[m]; !ok {
			inherited = append(inherited, m)
		}
	}
	fmt.Fprintf(e.Out, "OVERRIDES: %s\n", name)
	fmt.Fprintf(e.Out, "CHAIN: %s\n\n", strings.Join(reversed(chain), " > "))
	if len(overridden) > 0 {
		fmt.Fprintf(e.Out, "OVERRIDDEN (%d):\n\n", len(overridden))
		for _, m := range overridden {
			if po := parents[m]; mine[m] == po.sig {
				fmt.Fprintf(e.Out, "  %s (from %s)\n", mine[m], po.class)
			} else {
				fmt.Fprintf(e.Out, "  %s -> %s\n    parent: %s\n", mine[m], po.class, po.sig)
			}
		}
	}
	if len(added) > 0 {
		fmt.Fprintf(e.Out, "\nNEW METHODS (%d):\n\n", len(added))
		for _, m := range added {
			fmt.Fprintf(e.Out, "  %s\n", mine[m])
		}
	}
	if len(inherited) > 0 {
		fmt.Fprintf(e.Out, "\nINHERITED (not overridden) (%d):\n\n", len(inherited))
		e.printByOrigin(inherited, parents)
	}
	fmt.Fprintf(e.Out, "\nSummary: %d overridden, %d new, %d inherited\n", len(overridden), len(added), len(inherited))
	return nil
}

// printByOrigin prints methods in order under a "-- from <class> --" line
// each time the declaring class changes.
func (e *Env) printByOrigin(names []string, origins map[string]origin) {
	current := ""
	for i, m := range names {
		o := origins[m]
		if i == 0 || o.class != current {
			current = o.class
			fmt.Fprintf(e.Out, "  -- from %s --\n", o.class)
		}
		fmt.Fprintf(e.Out, "    %s\n", o.sig)
	}
}
