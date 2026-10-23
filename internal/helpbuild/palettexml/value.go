package palettexml

import "github.com/angeles725/niagara-go/internal/pyjson"

// Value is the palette-index.json tree of p: the root's children, each folder
//
//	{"folder": name, "type": type, "children": [...]}
//
// and each entry {"name", "type"} plus "properties" ([{"name", "type",
// "value"}], a key only when set) and "contains" when it has them.
func (p *Palette) Value() pyjson.Value { return children(p.Root) }

func children(n *Node) pyjson.Value {
	a := make([]pyjson.Value, len(n.Children))
	for i, c := range n.Children {
		a[i] = c.value()
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

func field(k string, v pyjson.Value) pyjson.Field { return pyjson.Field{Key: k, Value: v} }

func (n *Node) value() pyjson.Value {
	if n.Folder {
		return pyjson.ObjectOf(field("folder", pyjson.String(n.Name)), field("type", pyjson.String(n.Type)),
			field("children", children(n)))
	}
	o := pyjson.ObjectOf(field("name", pyjson.String(n.Name)), field("type", pyjson.String(n.Type)))
	if len(n.Properties) > 0 {
		props := make([]pyjson.Value, len(n.Properties))
		for i, pr := range n.Properties {
			v := pyjson.ObjectOf(field("name", pyjson.String(pr.Name)))
			for _, kv := range [][2]string{{"type", pr.Type}, {"value", pr.Value}} {
				if kv[1] != "" {
					v.Set(kv[0], pyjson.String(kv[1]))
				}
			}
			props[i] = v
		}
		o.Set("properties", pyjson.Value{Kind: pyjson.Array, A: props})
	}
	if len(n.Contains) > 0 {
		o.Set("contains", pyjson.StringList(n.Contains))
	}
	return o
}
