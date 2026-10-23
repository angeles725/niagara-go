package help

// The list views and the JSON of `module` (modules.go): --types, --agents,
// --permissions, --defs and --json.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// moduleDetailKeys maps a list flag to the record key it prints.
var moduleDetailKeys = map[string]string{"types": "types", "agents": "agents", "permissions": "java_permissions",
	"defs": "defs", "schemas": "schemas"}

// moduleMany answers a query that matched several modules: the listing (or
// the names as JSON); a list flag needs one module, so it is a miss.
func (e *Env) moduleMany(mods *index.Object, names []string, p ModuleParams) {
	if p.JSON && p.Detail == "" {
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"query", p.Name}, {"matches", names}}, pyjson.Indent2))
		return
	}
	e.moduleListing(mods, p.Name, names)
	if p.Detail != "" {
		e.Miss()
		fmt.Fprintf(e.Out, "\n--%s lists one module: name one of these.\n", p.Detail)
	}
}

// moduleOne prints one module: its summary, or one list, as text or JSON.
func (e *Env) moduleOne(mods *index.Object, name string, p ModuleParams) {
	r := mods.Object(name)
	switch {
	case p.JSON && p.Detail != "":
		key := moduleDetailKeys[p.Detail]
		obj := pyjson.Obj{{"module", name}, {key, get(r, key, []any{})}}
		if p.Detail == "permissions" {
			obj = append(obj, pyjson.KV{"permission_templates", get(r, "permission_templates", []any{})})
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(obj, pyjson.Indent2))
	case p.JSON:
		var rev []any
		for _, d := range reverseDependencies(mods, name) {
			rev = append(rev, pyjson.Obj{{"name", d[0]}, {"version", d[1]}})
		}
		if rev == nil {
			rev = []any{}
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"module", r}, {"reverse_dependencies", rev}}, pyjson.Indent2))
	case p.Detail == "types":
		e.moduleTypes(name, r)
	case p.Detail == "agents":
		e.moduleAgents(name, r)
	case p.Detail == "permissions":
		e.modulePermissions(name, r)
	case p.Detail == "schemas":
		e.moduleSchemas(name, r)
	case p.Detail == "defs":
		defs := objList(r, "defs")
		fmt.Fprintf(e.Out, "DEFS of %s (%d):\n", name, len(defs))
		for _, d := range defs {
			fmt.Fprintf(e.Out, "  %s = %s\n", getStr(d, "name", ""), getStr(d, "value", ""))
		}
	default:
		e.moduleSummary(mods, name)
	}
}

// typeSpec is the "part:Type" name Niagara gives a type of record r.
func typeSpec(r *index.Object, typeName string) string {
	part := getStr(r, "module", "")
	if part == "" {
		part = getStr(r, "name", "")
	}
	return part + ":" + typeName
}

func (e *Env) moduleTypes(name string, r *index.Object) {
	types := objList(r, "types")
	fmt.Fprintf(e.Out, "TYPES of %s (%d):\n", name, len(types))
	width := 0
	for _, t := range types {
		width = max(width, runeLen(typeSpec(r, getStr(t, "name", ""))))
	}
	for _, t := range types {
		fmt.Fprintf(e.Out, "  %s  %s\n", pycompat.LJust(typeSpec(r, getStr(t, "name", "")), width, ' '), getStr(t, "class", ""))
	}
}

func (e *Env) moduleAgents(name string, r *index.Object) {
	agents := objList(r, "agents")
	fmt.Fprintf(e.Out, "AGENTS of %s (%d):\n", name, len(agents))
	for _, a := range agents {
		fmt.Fprintf(e.Out, "  %s  %s\n", typeSpec(r, getStr(a, "type", "")), getStr(a, "class", ""))
		e.agentDetail(a, "    ")
	}
}

// agentDetail prints the types an agent is on and its optional attributes.
func (e *Env) agentDetail(a *index.Object, indent string) {
	on, _ := get(a, "on", nil).([]any)
	names := make([]string, len(on))
	for i, o := range on {
		names[i] = pyStr(o)
	}
	fmt.Fprintf(e.Out, "%son: %s\n", indent, strings.Join(names, ", "))
	if attrs := agentAttrs(a); attrs != "" {
		fmt.Fprintf(e.Out, "%s%s\n", indent, attrs)
	}
}
