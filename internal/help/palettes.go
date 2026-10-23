package help

// `palette` over indexes/palette-index.json (built by rebuild-all's
// palette_index_builder from every *.palette of the install's module jars):
// where a type, entry name or folder appears in the palettes Tridium ships,
// or (--module) one module's palette trees.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

const paletteIndexName = "palette-index.json"

// PaletteParams are the arguments of `palette`: Query is a type, name or
// folder, or with Module a module name; JSON prints the answer as JSON.
type PaletteParams struct {
	Query  string
	Module bool
	JSON   bool
}

// paletteNode is one folder or entry of a palette and its path in it.
type paletteNode struct {
	pal  *index.Object // the palette record
	path []string
	node *index.Object
}

func (n paletteNode) folder() bool { return n.node.Has("folder") }
func (n paletteNode) name() string {
	if n.folder() {
		return getStr(n.node, "folder", "")
	}
	return getStr(n.node, "name", "")
}

// palettes is the "palettes" list of palette-index.json (required).
func (e *Env) palettes() ([]*index.Object, error) {
	idx, err := e.loadIndex(paletteIndexName, true)
	if err != nil {
		return nil, err
	}
	return objList(idx, "palettes"), nil
}

// paletteNodes are every folder and entry of every palette, depth first.
func paletteNodes(pals []*index.Object) []paletteNode {
	var out []paletteNode
	var walk func(pal *index.Object, path []string, kids []*index.Object)
	walk = func(pal *index.Object, path []string, kids []*index.Object) {
		for _, k := range kids {
			n := paletteNode{pal, append(append([]string(nil), path...), ""), k}
			n.path[len(path)] = n.name()
			out = append(out, n)
			walk(pal, n.path, objList(k, "children"))
		}
	}
	for _, p := range pals {
		walk(p, nil, objList(p, "tree"))
	}
	return out
}

// paletteHit is a node a query matched; via is the type it holds that
// matched, "" when the node itself did.
type paletteHit struct {
	paletteNode
	via string
}

// typeMatches reports whether type spec "part:Type" matches query, which is
// "part:Type" or "Type", under match.
func typeMatches(spec, query string, match func(got, want string) bool) bool {
	part, name, _ := strings.Cut(spec, ":")
	if qp, qn, qualified := strings.Cut(query, ":"); qualified {
		return match(part, qp) && match(name, qn)
	}
	return match(name, query)
}

// resolvePalette is what a query names, in walk order: a node whose type,
// name or folder matches, else one holding a matching type; exactly, then in
// any letter case, then as a glob (the rounds of resolveTypes).
func resolvePalette(nodes []paletteNode, query string) []paletteHit {
	for _, match := range []func(got, want string) bool{
		func(got, want string) bool { return got == want },
		func(got, want string) bool { return textfold.Lower(got) == textfold.Lower(want) },
		fnmatch.Fnmatch,
		lowerGlob,
	} {
		var out []paletteHit
		for _, n := range nodes {
			if match(n.name(), query) || (!n.folder() && typeMatches(getStr(n.node, "type", ""), query, match)) {
				out = append(out, paletteHit{n, ""})
				continue
			}
			for _, t := range strList(n.node, "contains") {
				if typeMatches(t, query, match) {
					out = append(out, paletteHit{n, t})
					break
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// Palette is `palette`.
func (e *Env) Palette(p PaletteParams) error {
	pals, err := e.palettes()
	if err != nil {
		return err
	}
	if p.Module {
		return e.paletteModule(pals, p)
	}
	hits := resolvePalette(paletteNodes(pals), p.Query)
	if len(hits) == 0 {
		return e.reportMiss("palette", p.Query, "palettes")
	}
	if p.JSON {
		var list []any
		for _, h := range hits {
			kind := "entry"
			if h.folder() {
				kind = "folder"
			}
			o := pyjson.Obj{{"module", getStr(h.pal, "module", "")}, {"palette", getStr(h.pal, "path", "")},
				{"path", strings.Join(h.path, "/")}, {"type", getStr(h.node, "type", "")}, {"kind", kind}}
			if h.via != "" {
				o = append(o, pyjson.KV{"holds", h.via})
			}
			list = append(list, o)
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"query", p.Query}, {"hits", list}}, pyjson.Indent2))
		return nil
	}
	fmt.Fprintf(e.Out, "PALETTES with '%s' (%d):\n\n", p.Query, len(hits))
	width := 0
	for _, h := range hits {
		width = max(width, runeLen(getStr(h.pal, "module", "")))
	}
	for _, h := range hits {
		line := fmt.Sprintf("  %s  %s  %s", pycompat.LJust(getStr(h.pal, "module", ""), width, ' '), getStr(h.pal, "path", ""),
			strings.Join(h.path, "/"))
		if h.folder() {
			line += fmt.Sprintf("/  folder (%d items)", listLen(h.node, "children"))
		} else {
			line += "  " + getStr(h.node, "type", "")
		}
		if h.via != "" {
			line += "  (holds " + h.via + ")"
		}
		fmt.Fprintln(e.Out, line)
	}
	return nil
}
