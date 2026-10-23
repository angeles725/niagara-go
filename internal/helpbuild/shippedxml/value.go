package shippedxml

import "github.com/angeles725/niagara-go/internal/pyjson"

func field(k string, v pyjson.Value) pyjson.Field { return pyjson.Field{Key: k, Value: v} }

func list(n int, item func(i int) pyjson.Value) pyjson.Value {
	a := make([]pyjson.Value, n)
	for i := range a {
		a[i] = item(i)
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

// Value is the module-index.json record of a permissions template, every key
// always present:
//
//	{"path", "java_permissions": [{"profile", "class", "name", "action"}],
//	 "groups": [{"type", "requests": [{"name", "purpose_key",
//	   "parameters": [{"name", "value"}], "commented"}]}], "velocity": [...]}
func (p *Permissions) Value() pyjson.Value {
	str := pyjson.String
	return pyjson.ObjectOf(
		field("path", str(p.Path)),
		field("java_permissions", list(len(p.JavaPermissions), func(i int) pyjson.Value {
			j := p.JavaPermissions[i]
			return pyjson.ObjectOf(field("profile", str(j.Profile)), field("class", str(j.Class)),
				field("name", str(j.Name)), field("action", str(j.Action)))
		})),
		field("groups", list(len(p.Groups), func(i int) pyjson.Value {
			g := p.Groups[i]
			return pyjson.ObjectOf(field("type", str(g.Type)), field("requests", list(len(g.Requests), func(k int) pyjson.Value {
				r := g.Requests[k]
				return pyjson.ObjectOf(field("name", str(r.Name)), field("purpose_key", str(r.PurposeKey)),
					field("parameters", list(len(r.Parameters), func(n int) pyjson.Value {
						return pyjson.ObjectOf(field("name", str(r.Parameters[n][0])), field("value", str(r.Parameters[n][1])))
					})),
					field("commented", pyjson.Value{Kind: pyjson.Bool, B: r.Commented}))
			})))
		})),
		field("velocity", pyjson.StringList(p.Velocity)),
	)
}

// Value is the module-index.json record of a schema:
// {"path", "kind", "root", "target_namespace", "elements"}.
func (s Schema) Value() pyjson.Value {
	return pyjson.ObjectOf(field("path", pyjson.String(s.Path)), field("kind", pyjson.String(s.Kind)),
		field("root", pyjson.String(s.Root)), field("target_namespace", pyjson.String(s.TargetNamespace)),
		field("elements", pyjson.IntValue(int64(s.Elements))))
}
