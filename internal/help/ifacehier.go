package help

// Interface hierarchy, archetypes and orphan detection
// (interface_hierarchy_search.py). Every index here is required.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// resolveFold is _resolve: the key itself, else the first key equal to it
// ignoring case ("" if none).
func resolveFold(idx *index.Object, name string) string {
	if idx.Has(name) {
		return name
	}
	for _, k := range idx.Keys() {
		if lowerStr(k) == lowerStr(name) {
			return k
		}
	}
	return ""
}

// interfaceChain is _get_interface_chain: the extends chain, root first.
func interfaceChain(inh *index.Object, name string) []string {
	chain := []string{name}
	seen := map[string]bool{}
	for cur := name; cur != "" && !seen[cur]; {
		seen[cur] = true
		parent := truthyStr(getObj(inh, cur), "extends")
		if parent == "" {
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

// InterfaceHierarchyParams are the arguments of `interface-hierarchy`
// (and `hierarchy --interface`).
type InterfaceHierarchyParams struct {
	Name  string
	Depth int
	Chain bool
}

// InterfaceHierarchy shows an interface's extends chain (Chain) or its
// sub-interfaces and implementing classes (cmd_interface_hierarchy).
func (e *Env) InterfaceHierarchy(p InterfaceHierarchyParams) error {
	inh, err := e.loadIndex("inheritance.json", true)
	if err != nil {
		return err
	}
	ci, err := e.loadIndex("class-index.json", true)
	if err != nil {
		return err
	}
	resolved := resolveInheritanceKey(inh, p.Name)
	if resolved == "" {
		resolved = resolveFold(ci, p.Name)
	}
	if resolved == "" {
		if resolved = resolveFold(inh, "BI"+p.Name); resolved == "" {
			resolved = resolveFold(ci, "BI"+p.Name)
		}
	}
	if resolved == "" {
		return e.reportMiss("hierarchy", p.Name, "inheritance")
	}
	entry := getObj(inh, resolved)
	cls := getObj(ci, resolved)
	impl := strList(entry, "implements")
	if p.Chain {
		fmt.Fprintf(e.Out, "INTERFACE CHAIN: %s\n\n%s\n", resolved, strings.Join(interfaceChain(inh, resolved), " > "))
		if truthy(cls) {
			fmt.Fprintf(e.Out, "\nPackage: %s\n", getStr(cls, "package", "?"))
			fmt.Fprintf(e.Out, "Type: %s\n", getStr(cls, "type", "?"))
		}
		if len(impl) > 0 {
			fmt.Fprintf(e.Out, "Also extends: %s\n", strings.Join(impl, ", "))
		}
		return nil
	}
	fmt.Fprintf(e.Out, "INTERFACE HIERARCHY: %s (depth=%d)\n\n", resolved, p.Depth)
	if truthy(cls) {
		fmt.Fprintf(e.Out, "Package: %s\n", getStr(cls, "package", "?"))
	}
	if parent := get(entry, "extends", ""); truthy(parent) {
		fmt.Fprintf(e.Out, "Extends: %s\n", pyStr(parent))
	}
	if len(impl) > 0 {
		fmt.Fprintf(e.Out, "Also extends: %s\n", strings.Join(impl, ", "))
	}
	subs := strList(entry, "subclasses")
	if len(subs) == 0 {
		fmt.Fprintln(e.Out, "\nNo sub-interfaces or implementors found.")
		return nil
	}
	sort.Strings(subs)
	var ifaces, classes []string
	for _, s := range subs {
		if strings.HasPrefix(s, "BI") {
			ifaces = append(ifaces, s)
		} else {
			classes = append(classes, s)
		}
	}
	if len(ifaces) > 0 {
		fmt.Fprintf(e.Out, "\nSUB-INTERFACES (%d):\n\n", len(ifaces))
		for _, si := range ifaces {
			children := strList(getObj(inh, si), "subclasses")
			if len(children) > 0 {
				fmt.Fprintf(e.Out, "  %s (%d children)\n", si, len(children))
			} else {
				fmt.Fprintf(e.Out, "  %s\n", si)
			}
			if p.Depth > 1 { // one level of children under each sub-interface
				sort.Strings(children)
				for _, c := range children[:min(10, len(children))] {
					fmt.Fprintf(e.Out, "    %s\n", c)
				}
				if len(children) > 10 {
					fmt.Fprintf(e.Out, "    ... +%d more\n", len(children)-10)
				}
			}
		}
	}
	if len(classes) > 0 {
		fmt.Fprintf(e.Out, "\nIMPLEMENTING CLASSES (%d):\n\n", len(classes))
		for _, c := range classes {
			fmt.Fprintf(e.Out, "  %s (%s)\n", c, getStr(getObj(ci, c), "package", ""))
		}
	}
	return nil
}

type archetype struct{ name, base, desc string }

// archetypes is ARCHETYPES in its dict order (the ambiguity message keeps it).
var archetypes = []archetype{
	{"services", "BAbstractService", "Background services (alarm, driver, schedule, etc.)"},
	{"points", "BControlPoint", "Control points (numeric, boolean, string, enum writable/readonly)"},
	{"extensions", "BAbstractExtension", "Point extensions (alarm, history, scheduling, etc.)"},
	{"converters", "BConverter", "Type converters between Niagara types"},
	{"drivers", "BDeviceNetwork", "Device networks and protocol drivers"},
	{"devices", "BDevice", "Protocol device implementations"},
	{"servlets", "BWebServlet", "Web servlets for HTTP endpoints"},
	{"views", "BServletView", "Web views rendered by servlets"},
	{"agents", "BAgentMixin", "Agent mixins for component behavior"},
	{"schedules", "BWeeklySchedule", "Schedule components"},
	{"histories", "BHistoryService", "History-related services and components"},
	{"alarms", "BAlarmSourceExt", "Alarm source extensions"},
}

func findArchetype(name string) (archetype, bool) {
	for _, a := range archetypes {
		if a.name == name {
			return a, true
		}
	}
	return archetype{}, false
}

// descendants is count_descendants: every direct or indirect subclass of base.
func descendants(inh *index.Object, base string) []string {
	visited := map[string]bool{}
	var out []string
	queue := []string{base}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		if cur != base {
			out = append(out, cur)
		}
		for _, sub := range strList(getObj(inh, cur), "subclasses") {
			if !visited[sub] {
				queue = append(queue, sub)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ArchetypesParams are the arguments of `archetypes`; Type nil lists them all.
type ArchetypesParams struct{ Type *string }

// Archetypes lists the archetype categories with their class counts, or the
// classes of one archetype grouped by the last package segment (cmd_archetypes).
func (e *Env) Archetypes(p ArchetypesParams) error {
	inh, err := e.loadIndex("inheritance.json", true)
	if err != nil {
		return err
	}
	ci, err := e.loadIndex("class-index.json", true)
	if err != nil {
		return err
	}
	if inh.Len() == 0 {
		q := ""
		if p.Type != nil {
			q = *p.Type
		}
		return e.reportMiss("archetypes", q, "inheritance")
	}
	if p.Type == nil {
		return e.listArchetypes(inh)
	}
	want := lowerStr(*p.Type)
	key := strings.TrimRight(want, "s") + "s"
	if _, ok := findArchetype(key); !ok {
		if _, ok := findArchetype(want); ok {
			key = want
		}
	}
	a, ok := findArchetype(key)
	if !ok {
		var matches []string
		for _, a := range archetypes {
			if strings.Contains(a.name, want) {
				matches = append(matches, a.name)
			}
		}
		switch len(matches) {
		case 1:
			a, _ = findArchetype(matches[0])
		case 0:
			names := make([]string, len(archetypes))
			for i, a := range archetypes {
				names[i] = a.name
			}
			sort.Strings(names)
			fmt.Fprintf(e.Out, "Unknown archetype '%s'. Available: %s\n", *p.Type, strings.Join(names, ", "))
			return nil
		default:
			fmt.Fprintf(e.Out, "Ambiguous archetype '%s'. Matches: %s\n", *p.Type, strings.Join(matches, ", "))
			return nil
		}
	}
	desc := descendants(inh, a.base)
	fmt.Fprintf(e.Out, "ARCHETYPE: %s (%s)\nBASE CLASS: %s\nTOTAL: %d classes\n\n",
		strings.ToUpper(a.name), a.desc, a.base, len(desc))
	var modules []string
	byModule := map[string][][2]string{}
	for _, cls := range desc {
		pkg := getStr(getObj(ci, cls), "package", "unknown")
		module := pkg[strings.LastIndex(pkg, ".")+1:]
		if _, seen := byModule[module]; !seen {
			modules = append(modules, module)
		}
		byModule[module] = append(byModule[module], [2]string{cls, pkg})
	}
	sort.Strings(modules)
	for _, m := range modules {
		fmt.Fprintf(e.Out, "  %s (%d)\n", m, len(byModule[m]))
		for _, c := range byModule[m] {
			fmt.Fprintf(e.Out, "    %s (%s)\n", c[0], c[1])
		}
		fmt.Fprintln(e.Out)
	}
	return nil
}

func (e *Env) listArchetypes(inh *index.Object) error {
	fmt.Fprintf(e.Out, "NIAGARA ARCHETYPES\n%s\n\n", strings.Repeat("=", 65))
	row := func(count, name, base, desc string) {
		fmt.Fprintf(e.Out, "  %s  %s  %s  %s\n", rjust(count, 5), ljust(name, 20), ljust(base, 20), desc)
	}
	row("count", "archetype", "base class", "description")
	row("-----", "---------", "----------", "-----------")
	sorted := append([]archetype(nil), archetypes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	total := 0
	for _, a := range sorted {
		n := len(descendants(inh, a.base))
		total += n
		row(fmt.Sprint(n), a.name, a.base, a.desc)
	}
	fmt.Fprintf(e.Out, "\n  Total: %d classes across %d archetypes\n", total, len(archetypes))
	fmt.Fprintln(e.Out, "\n  Usage: archetypes <type> (e.g. 'archetypes services')")
	return nil
}
