package bogwrite

import (
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

type link struct{ name, srcHandle, srcSlot, tgtSlot string }

// node is _Node: name and type are plan values (checked only when emitted, as in Python).
type node struct {
	name, typ pyjson.Value
	handle    string
	slots     []slotSpec
	children  []*node
	ws        pyjson.Value
	links     []link
	names     map[string]bool
}

type slotSpec struct {
	name string
	spec pyjson.Value
}

// splitType is _split_type: "module:Type".
func splitType(t pyjson.Value) (string, string, error) {
	ok, err := contains(t, ":")
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", raise("ValueError", "type must be module:Type, got %s", t.Repr())
	}
	s, err := asStr(t, "split")
	if err != nil {
		return "", "", err
	}
	mod, name, _ := strings.Cut(s, ":")
	return mod, name, nil
}

// lookup is `key in index` / index[key] for a plan value key.
func lookup(index map[string]*node, key pyjson.Value) (*node, bool, error) {
	switch key.Kind {
	case pyjson.Array, pyjson.Object:
		return nil, false, raise("TypeError", "cannot use '%s' as a dict key (unhashable type: '%s')", key.TypeName(), key.TypeName())
	case pyjson.Str:
		n, ok := index[key.S]
		return n, ok, nil
	}
	return nil, false, nil
}

// generateXML is generate_xml.
func generateXML(plan pyjson.Value) (string, error) {
	rootSpec, err := get(plan, "root", pyjson.Value{})
	if err != nil {
		return "", err
	}
	if !rootSpec.Truthy() {
		rootSpec = pyjson.NewObject()
		rootSpec.Set("name", str("Generated"))
		rootSpec.Set("type", str("baja:Folder"))
	}
	handle := 0
	newHandle := func() string { handle++; return strconv.Itoa(handle) }
	rname, err := subscript(rootSpec, "name")
	if err != nil {
		return "", err
	}
	rtype, err := subscript(rootSpec, "type")
	if err != nil {
		return "", err
	}
	root := &node{name: rname, typ: rtype, handle: newHandle(), names: map[string]bool{}}
	index := map[string]*node{"": root}
	if err := addComponents(plan, root, index, newHandle); err != nil {
		return "", err
	}
	if err := addLinks(plan, index); err != nil {
		return "", err
	}
	e := &emitter{declared: map[string]bool{"baja": true}}
	body, err := e.node(root, true)
	if err != nil {
		return "", err
	}
	return "<?xml version='1.0'?><bajaObjectGraph version='4.0'>" + body + "</bajaObjectGraph>", nil
}

func emptyList() pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{}} }

func addComponents(plan pyjson.Value, root *node, index map[string]*node, newHandle func() string) error {
	comps, err := get(plan, "components", emptyList())
	if err != nil {
		return err
	}
	list, err := iterate(comps)
	if err != nil {
		return err
	}
	for _, comp := range list {
		parentPath, err := get(comp, "parent", str(""))
		if err != nil {
			return err
		}
		parent, ok, err := lookup(index, parentPath)
		if err != nil {
			return err
		}
		if !ok {
			return raise("ValueError", "unknown parent %s", parentPath.Repr())
		}
		raw, err := subscript(comp, "name")
		if err != nil {
			return err
		}
		rawName, err := asStr(raw, "endswith")
		if err != nil {
			return err
		}
		name := rawName
		if strings.HasSuffix(rawName, "?") {
			if name, err = uniqueName(strings.TrimSuffix(rawName, "?"), parent.names); err != nil {
				return err
			}
		} else if parent.names[rawName] {
			under := parentPath
			if !parentPath.Truthy() {
				under = root.name
			}
			return raise("ValueError", "duplicate component name %s under %s", pyjson.StrRepr(rawName), under.Repr())
		}
		if !nameOK(name) {
			return raise("ValueError", "illegal slot name %s (escape with $xx first)", pyjson.StrRepr(name))
		}
		typ, err := subscript(comp, "type")
		if err != nil {
			return err
		}
		n := &node{name: str(name), typ: typ, handle: newHandle(), names: map[string]bool{}}
		if n.ws, err = get(comp, "ws", pyjson.Value{}); err != nil {
			return err
		}
		slots, err := get(comp, "slots", pyjson.Value{})
		if err != nil {
			return err
		}
		if slots.Truthy() {
			if slots.Kind != pyjson.Object {
				return raise("AttributeError", "'%s' object has no attribute 'items'", slots.TypeName())
			}
			for _, k := range slots.Keys {
				n.slots = append(n.slots, slotSpec{k, slots.M[k]})
			}
		}
		parent.names[name] = true
		parent.children = append(parent.children, n)
		key := name
		if parentPath.S != "" { // lookup accepted it, so it is a str
			key = parentPath.S + "/" + name
		}
		index[key] = n
	}
	return nil
}

func addLinks(plan pyjson.Value, index map[string]*node) error {
	links, err := get(plan, "links", emptyList())
	if err != nil {
		return err
	}
	list, err := iterate(links)
	if err != nil {
		return err
	}
	for _, l := range list {
		var ends [2][2]string
		var raw [2]pyjson.Value
		for i, key := range []string{"from", "to"} {
			if raw[i], err = subscript(l, key); err != nil {
				return err
			}
			s, err := asStr(raw[i], "rsplit")
			if err != nil {
				return err
			}
			j := strings.LastIndexByte(s, '.')
			if j < 0 {
				return raise("ValueError", "not enough values to unpack (expected 2, got 1)")
			}
			ends[i] = [2]string{s[:j], s[j+1:]}
		}
		src, okS := index[ends[0][0]]
		tgt, okT := index[ends[1][0]]
		if !okS || !okT {
			return raise("ValueError", "link endpoint not found: %s -> %s", raw[0].Repr(), raw[1].Repr())
		}
		taken := map[string]bool{}
		for _, c := range tgt.children {
			taken[c.name.S] = true
		}
		for k := range tgt.names {
			taken[k] = true
		}
		lname, err := uniqueName("Link", taken)
		if err != nil {
			return err
		}
		tgt.names[lname] = true
		tgt.links = append(tgt.links, link{lname, src.handle, ends[0][1], ends[1][1]})
	}
	return nil
}

// emitter writes the document, declaring each module prefix on first use.
type emitter struct{ declared map[string]bool }

// prefix is _prefix_for.
func (e *emitter) prefix(mod string) (string, string) {
	if mod == "baja" {
		return "b", ""
	}
	decl := ""
	if !e.declared[mod] {
		decl = mod + "=" + mod
	}
	e.declared[mod] = true
	return mod, decl
}

// typed is ' m=".." t="pfx:Name"' for a slot type.
func (e *emitter) typed(t pyjson.Value) (string, error) {
	if !t.Truthy() {
		return "", nil
	}
	mod, name, err := splitType(t)
	if err != nil {
		return "", err
	}
	pfx, decl := e.prefix(mod)
	out := ""
	if decl != "" {
		out = attr("m", decl)
	}
	return out + attr("t", pfx+":"+name), nil
}

func (e *emitter) scalarSlot(name string, spec pyjson.Value) (string, error) {
	n := attr("n", name)
	if spec.Kind != pyjson.Object {
		value := spec.Str()
		if spec.Kind == pyjson.Bool {
			value = map[bool]string{true: "true", false: "false"}[spec.B]
		}
		return "<p" + n + attr("v", value) + "/>", nil
	}
	t, _ := spec.Get("type")
	if children, ok := spec.Get("children"); ok {
		ty, err := e.typed(t)
		if err != nil {
			return "", err
		}
		if children.Kind != pyjson.Object {
			return "", raise("AttributeError", "'%s' object has no attribute 'items'", children.TypeName())
		}
		var b strings.Builder
		b.WriteString("<p" + n + ty + ">")
		for _, k := range children.Keys {
			s, err := e.scalarSlot(k, children.M[k])
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		}
		return b.String() + "</p>", nil
	}
	if t.Truthy() {
		ts, err := asStr(t, "lower")
		if err != nil {
			return "", err
		}
		if strings.HasSuffix(strings.ToLower(ts), "password") {
			return "", raise("ValueError", "refusing to emit a password value (%s)", name)
		}
	}
	ty, err := e.typed(t)
	if err != nil {
		return "", err
	}
	value, _ := get(spec, "value", str(""))
	return "<p" + n + ty + attr("v", value.Str()) + "/>", nil
}

func (e *emitter) node(n *node, isRoot bool) (string, error) {
	mod, name, err := splitType(n.typ)
	if err != nil {
		return "", err
	}
	pfx, decl := e.prefix(mod)
	if isRoot && mod == "baja" {
		decl = "b=baja"
	}
	var b strings.Builder
	an, err := attrOf("n", n.name)
	if err != nil {
		return "", err
	}
	b.WriteString("<p" + an + attr("h", n.handle))
	if decl != "" {
		b.WriteString(attr("m", decl))
	}
	b.WriteString(attr("t", pfx+":"+name) + ">")
	for _, s := range n.slots {
		out, err := e.scalarSlot(s.name, s.spec)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	if n.ws.Truthy() {
		items, err := iterate(n.ws)
		if err != nil {
			return "", err
		}
		coords := make([]string, len(items))
		for i, x := range items {
			if coords[i], err = pyIntOf(x); err != nil {
				return "", err
			}
		}
		b.WriteString(`<p n="wsAnnotation" t="b:WsAnnotation" v="` + strings.Join(coords, ",") + `"/>`)
	}
	for _, l := range n.links {
		b.WriteString("<p" + attr("n", l.name) + ` t="b:Link"><p n="sourceOrd" v="h:` + l.srcHandle + `"/>` +
			`<p n="sourceSlotName"` + attr("v", l.srcSlot) + `/><p n="targetSlotName"` + attr("v", l.tgtSlot) + `/></p>`)
	}
	for _, c := range n.children {
		out, err := e.node(c, false)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String() + "</p>", nil
}
