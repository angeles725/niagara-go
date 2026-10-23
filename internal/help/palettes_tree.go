package help

// `palette --module`: one module's palettes as trees.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// modulePalettes are the palettes of the modules a name resolves to: the
// exact module, else in any letter case, else a glob.
func modulePalettes(pals []*index.Object, query string) []*index.Object {
	for _, match := range []func(got, want string) bool{
		func(got, want string) bool { return got == want },
		func(got, want string) bool { return textfold.Lower(got) == textfold.Lower(want) },
		fnmatch.Fnmatch,
		lowerGlob,
	} {
		var out []*index.Object
		for _, p := range pals {
			if match(getStr(p, "module", ""), query) {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (e *Env) paletteModule(pals []*index.Object, p PaletteParams) error {
	mine := modulePalettes(pals, p.Query)
	if len(mine) == 0 {
		return e.reportMiss("palette --module", p.Query, "palettes")
	}
	if p.JSON {
		list := make([]any, len(mine))
		for i, m := range mine {
			list[i] = m
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"module", p.Query}, {"palettes", list}}, pyjson.Indent2))
		return nil
	}
	for i, m := range mine {
		if i > 0 {
			fmt.Fprintln(e.Out)
		}
		folders, _ := index.Int(get(m, "folders", nil))
		entries, _ := index.Int(get(m, "entries", nil))
		fmt.Fprintf(e.Out, "PALETTE: %s %s (%s; %d folders, %d entries)\n", getStr(m, "module", ""), getStr(m, "path", ""),
			getStr(m, "jar", ""), folders, entries)
		e.paletteTree(objList(m, "tree"), "  ")
	}
	return nil
}

// paletteTree prints folders with a trailing "/", entries with their type,
// properties ("name: type", "name = value") and the types they hold.
func (e *Env) paletteTree(nodes []*index.Object, indent string) {
	for _, n := range nodes {
		if n.Has("folder") {
			fmt.Fprintf(e.Out, "%s%s/\n", indent, getStr(n, "folder", ""))
			e.paletteTree(objList(n, "children"), indent+"  ")
			continue
		}
		fmt.Fprintf(e.Out, "%s%s  %s\n", indent, getStr(n, "name", ""), getStr(n, "type", ""))
		for _, pr := range objList(n, "properties") {
			line := indent + "  " + getStr(pr, "name", "")
			if t := getStr(pr, "type", ""); t != "" {
				line += ": " + t
			}
			if v := getStr(pr, "value", ""); v != "" {
				line += " = " + v
			}
			fmt.Fprintln(e.Out, line)
		}
		if holds := strList(n, "contains"); len(holds) > 0 {
			fmt.Fprintf(e.Out, "%s  holds: %s\n", indent, strings.Join(holds, ", "))
		}
	}
}
