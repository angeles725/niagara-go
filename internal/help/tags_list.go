package help

import (
	"fmt"
	"path"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// tagList is `tag --list`: every name once with its kind and the files
// (base names) defining it.
func (e *Env) tagList(idx *index.Object, all []*tagEntry, asJSON bool) error {
	kinds := map[string]int{}
	width, kindOf, filesOf := 0, make([]string, len(all)), make([]string, len(all))
	for i, t := range all {
		kinds[t.kind]++
		width = max(width, runeLen(t.spec()))
		var files []string
		for _, r := range t.rows {
			files = append(files, path.Base(getStr(r, "file", "")))
			kindOf[i] = orStr(r, "kind", "", truthyStr(r, "type"))
		}
		filesOf[i] = strings.Join(files, ", ")
	}
	if asJSON {
		var list []any
		for i, t := range all {
			list = append(list, pyjson.Obj{{"dict", t.dict}, {"name", t.name}, {"kind", t.kind}, {"value_kind", kindOf[i]},
				{"files", anyList(strings.Split(filesOf[i], ", "))}})
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"tags", list}, {"unreadable", anyList(objList(idx, "errors"))}}, pyjson.Indent2))
		return nil
	}
	fmt.Fprintf(e.Out, "TAGS in %s (%d: %d tags, %d tag groups, %d relations; from %d files):\n\n", tagIndexName,
		len(all), kinds["tag"], kinds["tag group"], kinds["relation"], listLen(idx, "files"))
	for i, t := range all {
		line := fmt.Sprintf("  %s  %s  %s", pycompat.LJust(t.spec(), width, ' '), pycompat.LJust(t.kind, 9, ' '), kindOf[i])
		fmt.Fprintf(e.Out, "%s  %s\n", strings.TrimRight(line, " "), filesOf[i])
	}
	e.printUnreadable(idx)
	return nil
}
