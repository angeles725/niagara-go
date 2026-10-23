package help

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// classDocs says whether class-index.json and source-index.json hold the class
// of a type: the key to look it up by, "" when absent.
func (e *Env) classDocs(class string) (javadoc, source string, err error) {
	pkg, simple := "", class
	if i := strings.LastIndex(class, "."); i >= 0 {
		pkg, simple = class[:i], class[i+1:]
	}
	ci, err := e.loadIndex("class-index.json", false)
	if err != nil {
		return "", "", err
	}
	if ci.Has(simple) {
		entries, err := expand(simple, ci.Object(simple))
		if err != nil {
			return "", "", err
		}
		for _, c := range entries {
			if c.qualified() == class {
				javadoc = simple
			}
		}
	}
	si, err := e.loadIndex("source-index.json", false)
	if err != nil {
		return "", "", err
	}
	for _, m := range sourceMatches(si, simple) {
		if getStr(m.data, "package", "") == pkg {
			source = m.key
			break
		}
	}
	return javadoc, source, nil
}

// agentAttrs is the "app X; default Y; requires permissions Z" of an agent.
func agentAttrs(a *index.Object) string {
	var attrs []string
	for _, kv := range [][2]string{{"app", "app %s"}, {"default", "default %s"}, {"required_permissions", "requires permissions %s"}} {
		if v := getStr(a, kv[0], ""); v != "" {
			attrs = append(attrs, fmt.Sprintf(kv[1], v))
		}
	}
	return strings.Join(attrs, "; ")
}

// typeDetail prints one type registration.
func (e *Env) typeDetail(mods *index.Object, h typeHit, asJSON bool) error {
	javadoc, source, err := e.classDocs(h.class())
	if err != nil {
		return err
	}
	on, self := agentsOn(mods, h.spec()), agentOf(h)
	pals, err := e.typePalettes(h.spec())
	if err != nil {
		return err
	}
	if asJSON {
		agents := []any{}
		for _, a := range on {
			agents = append(agents, pyjson.Obj{{"agent", typeSpec(a.rec, getStr(a.typ, "type", ""))},
				{"class", getStr(a.typ, "class", "")}, {"module", a.module}, {"record", a.typ}})
		}
		selfs := []any{}
		for _, a := range self {
			selfs = append(selfs, a)
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"type", h.spec()}, {"module", h.module},
			{"jar", getStr(h.rec, "jar", "")}, {"class", h.class()}, {"class_index", javadoc != ""},
			{"source_index", source != ""}, {"agents_on_type", agents}, {"agent_on", selfs},
			{"palettes", paletteJSON(pals)}}, pyjson.Indent2))
		return nil
	}
	out := e.Out
	fmt.Fprintf(out, "TYPE: %s\nMODULE: %s (%s)\nCLASS: %s\n", h.spec(), h.module, getStr(h.rec, "jar", ""), h.class())
	simple := h.class()[strings.LastIndex(h.class(), ".")+1:]
	if javadoc != "" {
		fmt.Fprintf(out, "DOCS: class-index.json has %s (nh class %s)\n", h.class(), javadoc)
	} else {
		fmt.Fprintf(out, "DOCS: not in class-index.json (no javadoc page for %s)\n", simple)
	}
	if source != "" {
		fmt.Fprintf(out, "SOURCE: source-index.json has %s (nh source %s)\n", h.class(), source)
	} else {
		fmt.Fprintln(out, "SOURCE: not in source-index.json")
	}
	if len(pals) > 0 {
		var where []string
		for _, p := range pals {
			w := p.where()
			if p.via != "" {
				w += " (holds it)"
			}
			where = append(where, w)
		}
		fmt.Fprintf(out, "PALETTES: %s  (nh palette %s)\n", strings.Join(where, "; "), h.spec())
	}
	fmt.Fprintf(out, "\nAGENTS ON THIS TYPE (%d):\n", len(on))
	for _, a := range on {
		fmt.Fprintf(out, "  %s  %s  (%s)\n", typeSpec(a.rec, getStr(a.typ, "type", "")), getStr(a.typ, "class", ""), a.module)
		if attrs := agentAttrs(a.typ); attrs != "" {
			fmt.Fprintf(out, "    %s\n", attrs)
		}
	}
	if len(self) > 0 {
		fmt.Fprintf(out, "\nAGENT ON (%d):\n", len(self))
		for _, a := range self {
			e.agentDetail(a, "    ")
		}
	}
	return nil
}

// paletteJSON is the "palettes" list of type --json.
func paletteJSON(pals []paletteHit) []any {
	out := []any{}
	for _, p := range pals {
		out = append(out, pyjson.Obj{{"module", getStr(p.pal, "module", "")}, {"palette", getStr(p.pal, "path", "")},
			{"path", strings.Join(p.path, "/")}, {"holds", p.via != ""}})
	}
	return out
}
