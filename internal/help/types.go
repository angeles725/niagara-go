package help

// `type` over indexes/module-index.json: the module that registers a Niagara
// type, its class, whether nh has its javadoc and source, and the agents
// (views, editors) registered on it.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// TypeParams are the arguments of `type`.
type TypeParams struct {
	Name string
	JSON bool
}

// typeHit is one <type> registration: its module name, module record and type.
type typeHit struct {
	module   string
	rec, typ *index.Object
}

func (h typeHit) spec() string  { return typeSpec(h.rec, getStr(h.typ, "name", "")) }
func (h typeHit) class() string { return getStr(h.typ, "class", "") }

// resolveTypes is the registrations a query names, in index order: "part:Type"
// or "Type", matched exactly, then in any letter case, then as a glob.
func resolveTypes(mods *index.Object, query string) []typeHit {
	part, name, qualified := strings.Cut(query, ":")
	if !qualified {
		name = query
	}
	var all []typeHit
	for _, k := range mods.Keys() {
		r := mods.Object(k)
		for _, t := range objList(r, "types") {
			all = append(all, typeHit{k, r, t})
		}
	}
	for i, match := range matchRounds {
		var out []typeHit
		for _, h := range all {
			hp, hn, _ := strings.Cut(h.spec(), ":")
			partOK := !qualified || hp == part || (i > 0 && textfold.Lower(hp) == textfold.Lower(part))
			if partOK && match(hn, name) {
				out = append(out, h)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// agentsOn are the agents of every module registered on the type spec.
func agentsOn(mods *index.Object, spec string) []typeHit {
	var out []typeHit
	for _, k := range mods.Keys() {
		r := mods.Object(k)
		for _, a := range objList(r, "agents") {
			on, _ := get(a, "on", nil).([]any)
			for _, o := range on {
				if pyStr(o) == spec {
					out = append(out, typeHit{k, r, a})
					break
				}
			}
		}
	}
	return out
}

// agentOf are the agent entries of the type itself (it is a view or editor).
func agentOf(h typeHit) []*index.Object {
	var out []*index.Object
	for _, a := range objList(h.rec, "agents") {
		if getStr(a, "type", "") == getStr(h.typ, "name", "") {
			out = append(out, a)
		}
	}
	return out
}

// Type is `type`: one registration in detail, a listing of several, or a miss.
func (e *Env) Type(p TypeParams) error {
	mods, err := e.modules()
	if err != nil {
		return err
	}
	hits := resolveTypes(mods, p.Name)
	switch {
	case len(hits) == 0:
		return e.reportMiss("type", p.Name, "types")
	case len(hits) > 1 && p.JSON:
		var rows []any
		for _, h := range hits {
			rows = append(rows, pyjson.Obj{{"type", h.spec()}, {"class", h.class()}, {"module", h.module}})
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"query", p.Name}, {"matches", rows}}, pyjson.Indent2))
	case len(hits) > 1:
		fmt.Fprintf(e.Out, "%d types matching '%s':\n\n", len(hits), p.Name)
		for _, h := range hits {
			fmt.Fprintf(e.Out, "  %s  %s  (%s)\n", h.spec(), h.class(), h.module)
		}
	default:
		return e.typeDetail(mods, hits[0], p.JSON)
	}
	return nil
}
