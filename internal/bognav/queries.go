package bognav

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Component queries: tree, slot, find, types, grep and diff (bognav/queries.py).

func (e *env) comp(h string) *bog.Comp {
	c, _ := e.bog.Comps.Get(h)
	return c
}

func lastPart(s, sep string) string { return s[strings.LastIndex(s, sep)+len(sep):] }

func cmdTree(e *env) (int, int) {
	want := e.str("type")
	depthGiven := e.ns.Get("depth") != nil
	maxDepth := e.ns.Int("depth")
	var rows []row
	var walk func(h string, depth int)
	walk = func(h string, depth int) {
		c := e.comp(h)
		if !depthGiven || depth <= maxDepth {
			if want == "" || c.Type == want || c.SimpleType() == lastPart(want, ":") {
				rows = append(rows, row{{"depth", depth}, {"handle", h}, {"name", c.Name}, {"type", c.Type}, {"path", c.Path}})
			}
		}
		if !depthGiven || depth < maxDepth || want != "" { // keep descending to find deeper matches when filtering
			for _, ch := range c.Children {
				walk(ch, depth+1)
			}
		}
	}
	for _, r := range e.bog.Roots {
		walk(r, 0)
	}
	if e.flag("json") {
		e.println(dumps(rowsAny(rows), 2))
		return 0, -1
	}
	if len(rows) == 0 {
		of := ""
		if want != "" {
			of = " of type " + want
		}
		e.println("(no components", of, " -- ", e.scope, ")")
	}
	for _, r := range rows {
		e.println("h:", ljust(r.s("handle"), 8), " ", strings.Repeat("  ", r.get("depth").(int)), r.s("name"), "  <", r.s("type"), ">")
	}
	return 0, -1
}

func rowsAny(rows []row) []any {
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r
	}
	return out
}

// defaultMark is "(default)", or "(default, transient)" for a TRANSIENT slot (never saved).
func defaultMark(d defaultRec) string {
	if strings.Contains(d.Flags, "t") {
		return "(default, transient)"
	}
	return "(default)"
}

// absentDefaults is _absent_defaults: the class key ("" for None), a note, and the declared
// properties the bog does not store on c (neither a value slot nor a child of that name).
func (e *env) absentDefaults(c *bog.Comp) (string, string, *bog.Ordered[defaultRec]) {
	key, props := componentDefaults(e.bog, c)
	none := &bog.Ordered[defaultRec]{}
	if catalog() == nil {
		return "", "(defaults: no organized/ tree; set NIAGARA_ORGANIZED)", none
	}
	if key == "" {
		module, _ := e.bog.Prefixes.Get(c.Pfx)
		if module == "" {
			module = c.Module
		}
		if module == "" {
			module = "?"
		}
		return "", fmt.Sprintf("(defaults: type %s not found in organized/%s)", c.Type, module), none
	}
	present := map[string]bool{}
	for n := range c.Present {
		present[n] = true
	}
	for _, n := range c.Slots.Keys() {
		present[n] = true
	}
	for _, ch := range c.Children {
		present[e.comp(ch).Name] = true
	}
	absent := &bog.Ordered[defaultRec]{}
	for n, d := range props.All() {
		if !present[n] {
			absent.Set(n, d)
		}
	}
	return key, "", absent
}

func defaultJSON(d defaultRec) row {
	return row{{"default", d.Default}, {"declared_by", d.Owner}, {"flags", d.Flags}, {"source", d.Source}}
}

func (e *env) childNames(c *bog.Comp) string {
	var names []string
	for _, ch := range c.Children {
		names = append(names, e.comp(ch).Name)
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func cmdSlot(e *env) (int, int) {
	src := sourceTypes(e.str("src"))
	ref := e.str("ref")
	c := e.bog.Resolve(ref)
	if c == nil {
		fmt.Fprintf(e.err, "bog-nav: cannot resolve %s\n", ref)
		return 0, cli.ExitNoMatch
	}
	noDefaults := e.flag("no_defaults")
	cDefaults := func() (string, string, *bog.Ordered[defaultRec]) {
		if noDefaults {
			return "", "", &bog.Ordered[defaultRec]{}
		}
		return e.absentDefaults(c)
	}
	if name := e.str("slotname"); name != "" {
		s, ok := c.Slots.Get(name)
		out := row{{"path", c.Path}, {"slot", name}}
		var d defaultRec
		hasD := false
		if ok {
			out = append(out, slotJSON(s)...)
		} else {
			_, _, absent := cDefaults()
			d, hasD = absent.Get(name)
		}
		if hasD {
			out = append(out, defaultJSON(d)...)
		}
		switch {
		case e.flag("json"):
			e.println(dumps(out, 2))
		case ok:
			k, note := slotWritability(s, name, src)
			e.println(c.Path, ".", name, "  type=", typeDisplay(s.Type, name, src), "  value=", pyStr(s.Value),
				childSuffix(s), "  flags=", s.Flags, "  write=", k, " [", note, "]")
		case hasD:
			e.println(c.Path, ".", name, "  value=", d.Default, " ", defaultMark(d), "  declared by ", d.Owner,
				"  flags=", d.Flags, "  [absent from the bog: the type's default]")
		default:
			e.println(c.Path, ": no direct slot '", name, "' (children: ", e.childNames(c), ")")
		}
		return 0, -1
	}
	key, note, absent := cDefaults()
	if e.flag("json") {
		slots := row{}
		for n, s := range c.Slots.All() {
			slots = append(slots, kv{n, slotJSON(s)})
		}
		children := []any{}
		for _, ch := range c.Children {
			cc := e.comp(ch)
			children = append(children, row{{"name", cc.Name}, {"handle", ch}, {"type", cc.Type}})
		}
		out := row{{"path", c.Path}, {"handle", c.Handle}, {"type", c.Type}, {"slots", slots},
			{"actions", orderedStrings(&c.Actions)}, {"children", children}}
		if !noDefaults {
			var tc any
			if key != "" {
				tc = key
			}
			defs := row{}
			for n, d := range absent.All() {
				defs = append(defs, kv{n, defaultJSON(d)})
			}
			out = append(out, kv{"type_class", tc}, kv{"defaults", defs})
		}
		e.println(dumps(out, 2))
		return 0, -1
	}
	e.println(c.Path, "  h:", c.Handle, "  <", c.Type, ">")
	for n, s := range c.Slots.All() {
		k, _ := slotWritability(s, n, src)
		e.println("  slot  ", n, " = ", pyStr(s.Value), childSuffix(s), "  <", typeDisplay(s.Type, n, src), ">  f=", s.Flags, "  [", k, "]")
	}
	for n, d := range absent.All() {
		e.println("  slot  ", n, " = ", d.Default, " ", defaultMark(d), "  <", d.Owner, ">  f=", d.Flags)
	}
	for n, f := range c.Actions.All() {
		e.println("  action  ", n, "  f=", f)
	}
	for _, ch := range c.Children {
		cc := e.comp(ch)
		e.println("  child  ", cc.Name, "  h:", ch, "  <", cc.Type, ">")
	}
	if note != "" {
		e.println("  ", note)
	}
	return 0, -1
}

// typeMatcher is _type_matcher: PFX:Type exactly, or the bare trailing Type name.
func typeMatcher(want string) func(string) bool {
	simple := lastPart(want, ":")
	return func(tp string) bool {
		return tp != "" && (tp == want || lastPart(tp, ":") == simple)
	}
}

type slotHit struct {
	c          *bog.Comp
	path, kind string
}

// slotTypeHits is _slot_type_hits: every handle-less typed slot matching isT (the `value`
// wrapper and any other stored value slot) — what the handle index alone cannot see.
func (e *env) slotTypeHits(isT func(string) bool) []slotHit {
	var out []slotHit
	for _, c := range e.bog.Comps.All() {
		if c.ValueType != "" && isT(c.ValueType) {
			out = append(out, slotHit{c, c.Path + ".value", c.ValueType})
		}
		for n, s := range c.Slots.All() {
			if isT(s.Type) {
				out = append(out, slotHit{c, c.Path + "." + n, s.Type})
			}
		}
	}
	return out
}

func (e *env) compsByPath() []*bog.Comp {
	var cs []*bog.Comp
	for _, c := range e.bog.Comps.All() {
		cs = append(cs, c)
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Path < cs[j].Path })
	return cs
}

func cmdFind(e *env) (int, int) {
	want := e.str("type")
	isT := typeMatcher(want)
	var rows []row
	for _, c := range e.compsByPath() {
		if isT(c.Type) {
			rows = append(rows, row{{"handle", c.Handle}, {"path", c.Path}, {"type", c.Type}, {"kind", "component"}})
		}
	}
	withSlots := e.flag("slots")
	if withSlots {
		var slotRows []row
		for _, h := range e.slotTypeHits(isT) {
			slotRows = append(slotRows, row{{"handle", h.c.Handle}, {"path", h.path}, {"type", h.kind}, {"kind", "slot"}})
		}
		sort.SliceStable(slotRows, func(i, j int) bool { return slotRows[i].s("path") < slotRows[j].s("path") })
		rows = append(rows, slotRows...)
	}
	if len(rows) == 0 && !withSlots {
		// silent-zero guard: a type that lives only as a handle-less slot (bac:BacnetBinaryPv)
		if hidden := len(e.slotTypeHits(isT)); hidden > 0 {
			e.println(fmt.Sprintf("(no COMPONENTS of type %s, but %d handle-less slot(s) of it exist "+
				"-- rerun with --slots to list them)", want, hidden))
			return 0, -1
		}
	}
	return e.render(rows, func(r row) string {
		return "h:" + ljust(r.s("handle"), 8) + " " + r.s("path") + "  <" + r.s("type") + ">  [" + r.s("kind") + "]"
	}, "(no components or slots of type "+want+")", []string{"handle", "path", "type", "kind"}), -1
}

// counter is a collections.Counter of strings; mostCommon keeps first-seen order for ties.
type counter struct {
	keys []string
	n    map[string]int
}

func (c *counter) add(k string) {
	if c.n == nil {
		c.n = map[string]int{}
	}
	if _, ok := c.n[k]; !ok {
		c.keys = append(c.keys, k)
	}
	c.n[k]++
}

func (c *counter) mostCommon() []string {
	out := append([]string(nil), c.keys...)
	sort.SliceStable(out, func(i, j int) bool { return c.n[out[i]] > c.n[out[j]] })
	return out
}

func cmdTypes(e *env) (int, int) {
	var comp counter
	for _, c := range e.bog.Comps.All() {
		comp.add(c.Type)
	}
	var rows []row
	for _, t := range comp.mostCommon() {
		rows = append(rows, row{{"count", comp.n[t]}, {"type", t}, {"kind", "component"}})
	}
	if e.flag("slots") {
		var slot counter
		for _, c := range e.bog.Comps.All() {
			if c.ValueType != "" {
				slot.add(c.ValueType)
			}
			for _, s := range c.Slots.All() {
				if s.Type != "" {
					slot.add(s.Type)
				}
			}
		}
		for _, t := range slot.mostCommon() {
			rows = append(rows, row{{"count", slot.n[t]}, {"type", t}, {"kind", "slot"}})
		}
	}
	if grep := e.str("type_filter"); grep != "" {
		g := textfold.Lower(grep)
		kept := rows[:0]
		for _, r := range rows {
			if strings.Contains(textfold.Lower(r.s("type")), g) {
				kept = append(kept, r)
			}
		}
		rows = kept
	}
	e.render(rows, func(r row) string {
		return rjust(r.s("count"), 8) + "  " + ljust(r.s("type"), 34) + " [" + r.s("kind") + "]"
	}, "(no components)", []string{"count", "type", "kind"})
	return 0, -1
}

func cmdGrep(e *env) (int, int) {
	rx, err := regexp.Compile("(?i)" + e.str("regex"))
	if err != nil {
		fmt.Fprintf(e.err, "bog-nav: invalid regex %s: %v\n", pyRepr(e.str("regex")), err)
		return 0, cli.ExitCannotSearch
	}
	var rows []row
	for _, c := range e.bog.Comps.All() {
		if rx.MatchString(c.Path) || rx.MatchString(c.Type) {
			rows = append(rows, row{{"kind", "comp"}, {"path", c.Path}, {"type", c.Type}, {"handle", c.Handle}})
		}
		for n, s := range c.Slots.All() {
			if rx.MatchString(n) {
				rows = append(rows, row{{"kind", "slot"}, {"path", c.Path + "." + n}, {"type", s.Type}, {"handle", c.Handle}})
			}
		}
	}
	return e.render(rows, func(r row) string {
		return ljust(r.s("kind"), 5) + " h:" + ljust(r.s("handle"), 8) + " " + r.s("path") + "  <" + r.s("type") + ">"
	}, "(no matches)", nil), -1
}

func cmdDiff(e *env) (int, int) {
	other, err := bog.Load(e.str("bogB"))
	if err != nil {
		fmt.Fprintf(e.err, "bog-nav: %s\n", err)
		return 0, bog.ExitBadInput
	}
	paths := func(b *bog.Bog) map[string]*bog.Comp {
		m := map[string]*bog.Comp{}
		for _, c := range b.Comps.All() {
			m[c.Path] = c
		}
		return m
	}
	a, b := paths(e.bog), paths(other)
	var added, removed, common []string
	for p := range b {
		if a[p] == nil {
			added = append(added, p)
		}
	}
	for p := range a {
		if b[p] == nil {
			removed = append(removed, p)
		} else {
			common = append(common, p)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(common)
	var changed []row
	for _, p := range common {
		names := map[string]bool{}
		for _, n := range a[p].Slots.Keys() {
			names[n] = true
		}
		for _, n := range b[p].Slots.Keys() {
			names[n] = true
		}
		sorted := make([]string, 0, len(names))
		for n := range names {
			sorted = append(sorted, n)
		}
		sort.Strings(sorted)
		for _, n := range sorted {
			va, vb := slotValue(a[p], n), slotValue(b[p], n)
			if (va == nil) != (vb == nil) || (va != nil && *va != *vb) {
				changed = append(changed, row{{"path", p + "." + n}, {"a", va}, {"b", vb}})
			}
		}
	}
	if e.flag("json") {
		e.println(dumps(row{{"added", nonNil(added)}, {"removed", nonNil(removed)}, {"changed", rowsAny(changed)}}, 2))
		return 0, -1
	}
	for _, p := range added {
		e.println("+ ", p, "  <", b[p].Type, ">")
	}
	for _, p := range removed {
		e.println("- ", p, "  <", a[p].Type, ">")
	}
	for _, r := range changed {
		e.println("~ ", r.s("path"), "  ", r.s("a"), " -> ", r.s("b"))
	}
	if len(added)+len(removed)+len(changed) == 0 {
		e.println("(identical component/slot sets)")
	}
	return 0, -1
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func slotValue(c *bog.Comp, name string) *string {
	if s, ok := c.Slots.Get(name); ok {
		return s.Value
	}
	return nil
}
