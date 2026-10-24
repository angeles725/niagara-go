package help

// `tag` over indexes/tag-index.json (built by rebuild-all's tag_index_builder
// from the tag dictionaries of haystack-rt and tagdictionary-rt): what a
// Haystack or Niagara tag, tag group or relation is, and how N4 applies it.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

const tagIndexName = "tag-index.json"

// TagParams are the arguments of `tag`: Query is a name, dict:name or glob;
// List lists every name instead; JSON prints the answer as JSON.
type TagParams struct {
	Query      string
	List, JSON bool
}

// tagEntry is one dictionary name and the rows defining it, in file order.
type tagEntry struct {
	dict, name, kind string // kind: "tag", "tag group" or "relation"
	rows             []*index.Object
}

func (t *tagEntry) spec() string { return t.dict + ":" + t.name }

// tagEntries are the names of the tags, groups and relations sections, each
// once, in the order they first appear.
func tagEntries(idx *index.Object) []*tagEntry {
	var out []*tagEntry
	seen := map[string]*tagEntry{}
	for _, sec := range [][2]string{{"tags", "tag"}, {"groups", "tag group"}, {"relations", "relation"}} {
		for _, r := range objList(idx, sec[0]) {
			key := sec[1] + "\x00" + getStr(r, "dict", "") + ":" + getStr(r, "name", "")
			if seen[key] == nil {
				seen[key] = &tagEntry{dict: getStr(r, "dict", ""), name: getStr(r, "name", ""), kind: sec[1]}
				out = append(out, seen[key])
			}
			seen[key].rows = append(seen[key].rows, r)
		}
	}
	return out
}

// tagLinks are the records naming an entry: the rules applying it, the
// groups and protos holding it and the Haystack 3 names migrating to it.
type tagLinks struct {
	rules, migrations []*index.Object
	groups, protos    []string
}

// hasWord reports whether the space-separated tag set s holds name (a
// "phase(A)" cell is the tag phase).
func hasWord(s, name string) bool {
	for _, w := range strings.Fields(s) {
		if b, _, _ := strings.Cut(w, "("); b == name {
			return true
		}
	}
	return false
}

func linksOf(idx *index.Object, t *tagEntry) tagLinks {
	var l tagLinks
	for _, r := range objList(idx, "rules") {
		if getStr(r, "dict", "") != t.dict {
			continue
		}
		hit := false
		for _, rt := range append(objList(r, "tags"), objList(r, "relations")...) {
			hit = hit || (t.kind != "tag group" && getStr(rt, "name", "") == t.name)
		}
		for _, g := range strList(r, "groups") {
			hit = hit || (t.kind == "tag group" && g == t.name)
		}
		if hit {
			l.rules = append(l.rules, r)
		}
	}
	if t.kind != "tag" {
		return l
	}
	for _, g := range objList(idx, "groups") {
		if getStr(g, "dict", "") == t.dict && hasWord(strings.Join(strList(g, "tags"), " "), t.name) {
			l.groups = append(l.groups, getStr(g, "name", ""))
		}
	}
	if t.dict != "hs" { // protos and migrations are Haystack's
		return l
	}
	for _, p := range objList(idx, "protos") {
		if hasWord(getStr(p, "proto", ""), t.name) {
			l.protos = append(l.protos, getStr(p, "proto", ""))
		}
	}
	for _, m := range objList(idx, "migrations") {
		if getStr(m, "from", "") == t.name || hasWord(strings.Join(strList(m, "to"), " "), t.name) {
			l.migrations = append(l.migrations, m)
		}
	}
	return l
}

// tagCell is a row value as text: a list joined with ", ", an object as
// name=value cells joined with "; ".
func tagCell(v any) string {
	switch v := v.(type) {
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = pyStr(x)
		}
		return strings.Join(parts, ", ")
	case *index.Object:
		var parts []string
		for _, k := range v.Keys() {
			parts = append(parts, k+"="+getStr(v, k, ""))
		}
		return strings.Join(parts, "; ")
	}
	return pyStr(v)
}

// anyList is xs as the []any pyjson dumps, [] when empty.
func anyList[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// tagProtosShown is how many protos a text answer prints (--json lists all).
const tagProtosShown = 5

func (e *Env) printTag(t *tagEntry, l tagLinks) {
	fmt.Fprintf(e.Out, "TAG %s  (%s; %d definitions)\n", t.spec(), t.kind, len(t.rows))
	for _, r := range t.rows {
		fmt.Fprintf(e.Out, "  %s\n", getStr(r, "file", ""))
		for _, k := range r.Keys() {
			if k != "dict" && k != "name" && k != "file" {
				v, _ := r.Get(k)
				pad := "\n" + strings.Repeat(" ", 6+runeLen(k))
				fmt.Fprintf(e.Out, "    %s: %s\n", k, strings.ReplaceAll(tagCell(v), "\n", pad))
			}
		}
	}
	if len(l.rules) > 0 {
		fmt.Fprintf(e.Out, "  N4 rules (%d):\n", len(l.rules))
		for _, r := range l.rules {
			fmt.Fprintf(e.Out, "    %s  %s  (%s)\n", getStr(r, "name", ""), getStr(r, "condition", ""), getStr(r, "file", ""))
		}
	}
	if len(l.groups) > 0 {
		fmt.Fprintf(e.Out, "  tag groups (%d): %s\n", len(l.groups), strings.Join(l.groups, ", "))
	}
	if len(l.protos) > 0 {
		fmt.Fprintf(e.Out, "  protos (%d):\n", len(l.protos))
		for _, p := range l.protos[:min(tagProtosShown, len(l.protos))] {
			fmt.Fprintf(e.Out, "    %s\n", p)
		}
		if n := len(l.protos) - tagProtosShown; n > 0 {
			fmt.Fprintf(e.Out, "    ... and %d more (--json lists them all)\n", n)
		}
	}
	if len(l.migrations) > 0 {
		fmt.Fprintf(e.Out, "  Haystack 3 migrations (%d):\n", len(l.migrations))
		for _, m := range l.migrations {
			fmt.Fprintf(e.Out, "    %s (%s) -> %s (%s)\n", getStr(m, "from", ""), getStr(m, "fromType", ""),
				strings.Join(strList(m, "to"), ", "), getStr(m, "toType", ""))
		}
	}
}

// printUnreadable names the files an install-fed index could not read, so
// an answer never passes for the whole install when it is not.
func (e *Env) printUnreadable(idx *index.Object) {
	errs := objList(idx, "errors")
	if len(errs) == 0 {
		return
	}
	fmt.Fprintf(e.Out, "\nUNREADABLE (%d):", len(errs))
	for _, x := range errs {
		where := getStr(x, "jar", "")
		if p := getStr(x, "path", ""); p != "" {
			where += "!" + p
		}
		fmt.Fprintf(e.Out, " %s: %s\n", where, getStr(x, "error", ""))
	}
}

// Tag is `tag`.
func (e *Env) Tag(p TagParams) error {
	idx, err := e.loadIndex(tagIndexName, true)
	if err != nil {
		return err
	}
	all := tagEntries(idx)
	switch {
	case p.List:
		return e.tagList(idx, all, p.JSON)
	case p.Query == "":
		return usageError("tag needs a name, dict:name or glob, or --list")
	}
	var hits []*tagEntry
	for _, match := range matchRounds {
		for _, t := range all {
			if typeMatches(t.spec(), p.Query, match) {
				hits = append(hits, t)
			}
		}
		if len(hits) > 0 {
			break
		}
	}
	if len(hits) == 0 {
		return e.reportMiss("tag", p.Query, "tags")
	}
	if p.JSON {
		var list []any
		for _, t := range hits {
			l := linksOf(idx, t)
			list = append(list, pyjson.Obj{{"dict", t.dict}, {"name", t.name}, {"kind", t.kind}, {"definitions", anyList(t.rows)},
				{"rules", anyList(l.rules)}, {"groups", anyList(l.groups)}, {"protos", anyList(l.protos)}, {"migrations", anyList(l.migrations)}})
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"query", p.Query}, {"hits", list},
			{"unreadable", anyList(objList(idx, "errors"))}}, pyjson.Indent2))
		return nil
	}
	fmt.Fprintf(e.Out, "TAGS with '%s' (%d):\n", p.Query, len(hits))
	for _, t := range hits {
		fmt.Fprintln(e.Out)
		e.printTag(t, linksOf(idx, t))
	}
	e.printUnreadable(idx)
	return nil
}

// namesType reports whether condition cond holds "isType spec" with spec
// whole (isType control:ControlPointExt does not name control:ControlPoint).
func namesType(cond, spec string) bool {
	needle := "isType " + spec
	for rest := cond; ; {
		i := strings.Index(rest, needle)
		if i < 0 {
			return false
		}
		rest = rest[i+len(needle):]
		if rest == "" || !(rest[0] == '_' || rest[0] == ':' || rest[0] >= '0' && rest[0] <= '9' ||
			rest[0]|0x20 >= 'a' && rest[0]|0x20 <= 'z') {
			return true
		}
	}
}

// typeTags are the dictionary names (dict:name) given to an object of type
// spec for `type`: those a rule applies when its condition names isType spec,
// and the tag groups and tags whose validity does (an absent tag-index.json
// gives none). Only conditions naming the type itself count, not a supertype.
func (e *Env) typeTags(spec string) ([]string, error) {
	idx, err := e.loadIndex(tagIndexName, false)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	add := func(dict, name string) {
		if k := dict + ":" + name; !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, r := range objList(idx, "rules") {
		if namesType(getStr(r, "condition", ""), spec) {
			for _, t := range append(objList(r, "tags"), objList(r, "relations")...) {
				add(getStr(r, "dict", ""), getStr(t, "name", ""))
			}
			for _, g := range strList(r, "groups") {
				add(getStr(r, "dict", ""), g)
			}
		}
	}
	for _, sec := range []string{"groups", "tags"} {
		for _, r := range objList(idx, sec) {
			v := getObj(r, "validity")
			if v == nil {
				continue
			}
			for _, k := range v.Keys() {
				if c := getStr(v, k, ""); namesType(c, spec) || (k == "isType" && c == spec) {
					add(getStr(r, "dict", ""), getStr(r, "name", ""))
				}
			}
		}
	}
	return out, nil
}
