package modulexml

import "github.com/angeles725/niagara-go/internal/pyjson"

// Value is the module-index.json record of m: every key always present, an
// agent's app, default and required_permissions only when the descriptor sets them.
func (m *Module) Value() pyjson.Value {
	str := pyjson.String
	list := func(n int, item func(i int) pyjson.Value) pyjson.Value {
		a := make([]pyjson.Value, n)
		for i := range a {
			a[i] = item(i)
		}
		return pyjson.Value{Kind: pyjson.Array, A: a}
	}
	f := func(k string, v pyjson.Value) pyjson.Field { return pyjson.Field{Key: k, Value: v} }
	return pyjson.ObjectOf(
		f("name", str(m.Name)), f("module", str(m.Part)), f("runtime_profile", str(m.Profile)),
		f("vendor", str(m.Vendor)), f("version", str(m.Version)), f("description", str(m.Description)),
		f("dependencies", list(len(m.Dependencies), func(i int) pyjson.Value {
			d := m.Dependencies[i]
			return pyjson.ObjectOf(f("name", str(d.Name)), f("vendor", str(d.Vendor)), f("version", str(d.Version)))
		})),
		f("types", list(len(m.Types), func(i int) pyjson.Value {
			return pyjson.ObjectOf(f("name", str(m.Types[i].Name)), f("class", str(m.Types[i].Class)))
		})),
		f("agents", list(len(m.Agents), func(i int) pyjson.Value { return m.Agents[i].value() })),
		f("java_permissions", list(len(m.JavaPermissions), func(i int) pyjson.Value {
			p := m.JavaPermissions[i]
			return pyjson.ObjectOf(f("profile", str(p.Profile)), f("class", str(p.Class)), f("name", str(p.Name)),
				f("action", str(p.Action)))
		})),
		f("defs", list(len(m.Defs), func(i int) pyjson.Value {
			return pyjson.ObjectOf(f("name", str(m.Defs[i].Name)), f("value", str(m.Defs[i].Value)))
		})),
		f("adapters", list(len(m.Adapters), func(i int) pyjson.Value {
			a := m.Adapters[i]
			return pyjson.ObjectOf(f("type", str(a.Type)), f("from", str(a.From)), f("to", str(a.To)))
		})),
	)
}

func (a Agent) value() pyjson.Value {
	o := pyjson.ObjectOf(pyjson.Field{Key: "type", Value: pyjson.String(a.Type)},
		pyjson.Field{Key: "class", Value: pyjson.String(a.Class)}, pyjson.Field{Key: "on", Value: pyjson.StringList(a.On)})
	for _, kv := range [][2]string{{"app", a.App}, {"default", a.Default}, {"required_permissions", a.Perms}} {
		if kv[1] != "" {
			o.Set(kv[0], pyjson.String(kv[1]))
		}
	}
	return o
}
