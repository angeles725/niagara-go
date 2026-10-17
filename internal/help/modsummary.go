package help

// Module overview (module_summary.py): classes, packages, slots, own methods,
// module dependencies, key classes and external supertypes of one module.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// countedName is a (name, count) pair of a dict Python sorts by count only.
type countedName struct {
	name  string
	count int
}

// counter is a dict of counts that remembers insertion order, so a stable
// sort by count keeps Python's order for equal counts.
type counter struct {
	order []string
	n     map[string]int
}

func (c *counter) add(k string) {
	if c.n == nil {
		c.n = map[string]int{}
	}
	if _, ok := c.n[k]; !ok {
		c.order = append(c.order, k)
	}
	c.n[k]++
}

// byCount is sorted(d.items(), key=lambda x: -x[1]).
func (c *counter) byCount() []countedName {
	out := make([]countedName, len(c.order))
	for i, k := range c.order {
		out[i] = countedName{k, c.n[k]}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].count > out[j].count })
	return out
}

// slotCount is the number of properties, actions and topics of a class.
func slotCount(slots *index.Object, cls string) (props, actions, topics int) {
	d := getObj(slots, cls)
	return listLen(d, "properties"), listLen(d, "actions"), listLen(d, "topics")
}

// ModuleSummary is cmd_module_summary.
func (e *Env) ModuleSummary(name string) error {
	idx := map[string]*index.Object{}
	for _, n := range []string{"source-index.json", "xref-index.json", "slots-index.json",
		"method-index.json", "inheritance.json"} {
		o, err := e.loadIndex(n, true)
		if err != nil {
			return err
		}
		idx[n] = o
	}
	xref, slots, inh := idx["xref-index.json"], idx["slots-index.json"], idx["inheritance.json"]
	m := newSourceModules(idx["source-index.json"])
	if _, ok := m.classes[name]; !ok {
		matches := m.partial(name)
		switch {
		case len(matches) == 0:
			if err := e.reportMiss("module-summary", name, "modules"); err != nil {
				return err
			}
			if len(m.order) <= 50 {
				fmt.Fprintln(e.Out, "\nAvailable modules:")
				m.printList(e, m.order)
			} else {
				fmt.Fprintln(e.Out, "Use 'module-deps' to list all modules.")
			}
			return nil
		case len(matches) == 1:
			name = matches[0]
		default:
			exact := ""
			for _, mod := range matches {
				if exact == "" && lowerStr(mod) == lowerStr(name) {
					exact = mod
				}
			}
			if exact == "" {
				fmt.Fprintf(e.Out, "%d modules match '%s':\n", len(matches), name)
				m.printList(e, matches)
				return nil
			}
			name = exact
		}
	}
	classes := m.classes[name]
	names := make([]string, len(classes))
	own := map[string]bool{}
	for i, c := range classes {
		names[i] = c.name
		own[c.name] = true
	}
	// Python iterates set(names): equal counts below keep that order.
	setOrder := pySetOrder(names)

	totalLines := 0
	packages := map[string]bool{}
	var props, actions, topics, withSlots int
	for _, c := range classes {
		totalLines += pyInt(get(c.data, "lines", nil))
		if pkg := truthyStr(c.data, "package"); pkg != "" {
			packages[pkg] = true
		}
		p, a, t := slotCount(slots, c.name)
		props, actions, topics = props+p, actions+a, topics+t
		if p+a+t > 0 {
			withSlots++
		}
	}
	ownMethods := 0
	methodsBy := map[string]int{}
	methods := idx["method-index.json"]
	for _, k := range methods.Keys() {
		for _, me := range objList(methods, k) {
			cls := getStr(me, "class", "")
			if own[cls] && getStr(me, "section", "") == "methods" {
				ownMethods++
				methodsBy[cls]++
			}
		}
	}
	var deps, dependents counter
	var key []countedName
	bases := map[string]bool{}
	ifaces := map[string]bool{}
	for _, cls := range setOrder {
		x := getObj(xref, cls)
		for _, used := range strList(x, "uses") {
			if other := m.moduleOf[used]; other != "" && other != name {
				deps.add(other)
			}
		}
		for _, user := range strList(x, "used_by") {
			if other := m.moduleOf[user]; other != "" && other != name {
				dependents.add(other)
			}
		}
		if n := listLen(x, "used_by"); n > 0 {
			key = append(key, countedName{cls, n})
		}
		ih := getObj(inh, cls)
		if ext := truthyStr(ih, "extends"); ext != "" && !own[ext] {
			bases[ext] = true
		}
		for _, i := range strList(ih, "implements") {
			ifaces[i] = true
		}
	}
	sort.SliceStable(key, func(i, j int) bool { return key[i].count > key[j].count })

	out := e.Out
	fmt.Fprintf(out, "MODULE SUMMARY: %s\n%s\n", name, strings.Repeat("=", 60))
	fmt.Fprintf(out, "  Classes:     %d\n  Source lines: %s\n  Packages:    %d\n", len(classes), thousands(totalLines), len(packages))
	fmt.Fprintf(out, "  Slots:       %d props, %d actions, %d topics (%d classes)\n", props, actions, topics, withSlots)
	fmt.Fprintf(out, "  Methods:     %d own methods (%d classes)\n", ownMethods, len(methodsBy))
	fmt.Fprintf(out, "  Depends on:  %d modules\n  Depended by: %d modules\n", len(deps.order), len(dependents.order))
	if len(packages) > 0 {
		fmt.Fprintf(out, "\nPACKAGES (%d):\n\n", len(packages))
		for _, pkg := range sortedKeys(packages) {
			n := 0
			for _, c := range classes {
				if s, ok := get(c.data, "package", nil).(string); ok && s == pkg {
					n++
				}
			}
			fmt.Fprintf(out, "  %s (%d classes)\n", pkg, n)
		}
	}
	e.printModuleClasses(classes, slots, xref, methodsBy)
	if len(deps.order) > 0 {
		fmt.Fprintf(out, "\nDEPENDS ON (%d modules):\n\n", len(deps.order))
		for _, d := range deps.byCount() {
			fmt.Fprintf(out, "  %s (%d imports)\n", d.name, d.count)
		}
	}
	if len(dependents.order) > 0 {
		fmt.Fprintf(out, "\nDEPENDED ON BY (%d modules):\n\n", len(dependents.order))
		all := dependents.byCount()
		for _, d := range all[:min(len(all), 15)] {
			fmt.Fprintf(out, "  %s (%d imports)\n", d.name, d.count)
		}
		if len(all) > 15 {
			fmt.Fprintf(out, "  ... +%d more\n", len(all)-15)
		}
	}
	if len(key) > 0 {
		fmt.Fprint(out, "\nKEY CLASSES (most referenced):\n\n")
		for _, k := range key[:min(len(key), 10)] {
			fmt.Fprintf(out, "  %s (%d references)\n", k.name, k.count)
		}
	}
	if len(bases) > 0 {
		fmt.Fprint(out, "\nEXTERNAL BASE CLASSES:\n\n")
		for _, bc := range sortedKeys(bases) {
			n := 0
			for _, cls := range setOrder {
				if s, ok := get(getObj(inh, cls), "extends", nil).(string); ok && s == bc {
					n++
				}
			}
			fmt.Fprintf(out, "  %s (%d subclasses)\n", bc, n)
		}
	}
	var ext []string
	for _, i := range sortedKeys(ifaces) {
		if !own[i] {
			ext = append(ext, i)
		}
	}
	if len(ext) > 0 {
		fmt.Fprint(out, "\nEXTERNAL INTERFACES USED:\n\n")
		for _, i := range ext[:min(len(ext), 20)] {
			fmt.Fprintf(out, "  %s\n", i)
		}
		if len(ext) > 20 {
			fmt.Fprintf(out, "  ... +%d more\n", len(ext)-20)
		}
	}
	return nil
}

// printModuleClasses lists the classes by package (missing: "(default)"),
// each with its lines and markers: slots, own methods, references.
func (e *Env) printModuleClasses(classes []namedEntry, slots, xref *index.Object, methodsBy map[string]int) {
	fmt.Fprintf(e.Out, "\nCLASSES (%d):\n\n", len(classes))
	byPkg := map[string][]namedEntry{}
	for _, c := range classes {
		pkg := getStr(c.data, "package", "(default)")
		byPkg[pkg] = append(byPkg[pkg], c)
	}
	for _, pkg := range sortedKeys(byPkg) {
		group := append([]namedEntry(nil), byPkg[pkg]...)
		sortByName(group)
		for _, c := range group {
			var markers []string
			if p, a, t := slotCount(slots, c.name); p+a+t > 0 {
				markers = append(markers, fmt.Sprintf("%ds", p+a+t))
			}
			if n := methodsBy[c.name]; n > 0 {
				markers = append(markers, fmt.Sprintf("%dm", n))
			}
			if n := listLen(getObj(xref, c.name), "used_by"); n > 0 {
				markers = append(markers, fmt.Sprintf("%drefs", n))
			}
			tail := ""
			if len(markers) > 0 {
				tail = " [" + strings.Join(markers, ", ") + "]"
			}
			fmt.Fprintf(e.Out, "  %s (%s lines)%s\n", c.name, pyStr(get(c.data, "lines", 0)), tail)
		}
	}
}
