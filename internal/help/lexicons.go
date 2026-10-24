package help

// `lexicon` over indexes/lexicon-index.json (built by rebuild-all's
// lexicon_index_builder from every lexicon file of the install's module jars):
// what a lexicon key says in each language, the base value a translation
// overrides next to it.

import (
	"fmt"
	"sort"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

const lexiconIndexName = "lexicon-index.json"

// LexiconParams are the arguments of `lexicon`: Query is a key or glob; Module
// keeps one module's lexicon (name or glob); Lang one language code, "base"
// (the module's own files) or "all" ("" is all); JSON prints the answer as JSON.
type LexiconParams struct {
	Query, Module, Lang string
	JSON                bool
}

// lexiconFile is one files row of the index.
type lexiconFile struct {
	jar, path, module, language, overrides string
	entries                                []any // ["key", "value"] pairs
}

// lexModule is the module whose lexicon the file belongs to: the one a
// translation overrides, else its own.
func (f *lexiconFile) lexModule() string {
	if f.overrides != "" {
		return f.overrides
	}
	return f.module
}

func (f *lexiconFile) langLabel() string {
	if f.language == "" {
		return "base"
	}
	return f.language
}

// lexiconFiles are the files rows of idx.
func lexiconFiles(idx *index.Object) []*lexiconFile {
	var out []*lexiconFile
	for _, r := range objList(idx, "files") {
		entries, _ := get(r, "entries", nil).([]any)
		out = append(out, &lexiconFile{getStr(r, "jar", ""), getStr(r, "path", ""), getStr(r, "module", ""),
			getStr(r, "language", ""), getStr(r, "overrides", ""), entries})
	}
	return out
}

// pair is one entry of a file as its key and value.
func pair(e any) (key, value string) {
	kv, _ := e.([]any)
	if len(kv) == 2 {
		key, value = pyStr(kv[0]), pyStr(kv[1])
	}
	return key, value
}

// lexiconRow is one entry of one file; matched is false for a base value
// shown only as what a matching translation overrides.
type lexiconRow struct {
	f       *lexiconFile
	value   string
	matched bool
}

// lexiconKey is one module's key and its rows, base values first.
type lexiconKey struct {
	module, key string
	rows        []lexiconRow
}

// nameMatches reports whether got is want in any letter case or as a glob.
func nameMatches(got, want string) bool {
	return textfold.Lower(got) == textfold.Lower(want) || fnmatch.Fnmatch(got, want)
}

// langSelected reports whether a file of language lang is in the selection
// sel ("all", "base" or a language code, in any letter case).
func langSelected(sel, lang string) bool {
	switch textfold.Lower(sel) {
	case "all":
		return true
	case "base":
		return lang == ""
	}
	return lang != "" && textfold.Lower(lang) == textfold.Lower(sel)
}

// lexiconKeys groups the matching entries of files by module and key (in the
// order they first appear) and adds the base rows of every key a translation
// matched; matches is how many entries matched.
func lexiconKeys(files []*lexiconFile, hit func(f *lexiconFile, key, value string) bool) (keys []*lexiconKey, matches int) {
	byKey := map[string]*lexiconKey{}
	for _, f := range files {
		for _, e := range f.entries {
			key, value := pair(e)
			if !hit(f, key, value) {
				continue
			}
			id := f.lexModule() + "\x00" + key
			if byKey[id] == nil {
				byKey[id] = &lexiconKey{module: f.lexModule(), key: key}
				keys = append(keys, byKey[id])
			}
			byKey[id].rows = append(byKey[id].rows, lexiconRow{f, value, true})
			matches++
		}
	}
	for _, f := range files {
		for _, e := range f.entries {
			key, value := pair(e)
			if k := byKey[f.lexModule()+"\x00"+key]; f.language == "" && k != nil && !k.hasRow(f) {
				k.rows = append(k.rows, lexiconRow{f, value, false})
			}
		}
	}
	for _, k := range keys {
		sort.SliceStable(k.rows, func(i, j int) bool { return k.rows[i].f.language == "" && k.rows[j].f.language != "" })
	}
	return keys, matches
}

func (k *lexiconKey) hasRow(f *lexiconFile) bool {
	for _, r := range k.rows {
		if r.f == f {
			return true
		}
	}
	return false
}

// lexiconKeysShown is how many keys a text answer prints (--json lists all).
var lexiconKeysShown = 25

// Lexicon is `lexicon`.
func (e *Env) Lexicon(p LexiconParams) error {
	if p.Query == "" {
		return usageError("lexicon needs a key or glob")
	}
	idx, err := e.loadIndex(lexiconIndexName, true)
	if err != nil {
		return err
	}
	if p.Lang == "" {
		p.Lang = "all"
	}
	var files []*lexiconFile
	for _, f := range lexiconFiles(idx) {
		if p.Module == "" || nameMatches(f.lexModule(), p.Module) {
			files = append(files, f)
		}
	}
	var keys []*lexiconKey
	var matches int
	for _, match := range matchRounds {
		keys, matches = lexiconKeys(files, func(f *lexiconFile, key, _ string) bool {
			return langSelected(p.Lang, f.language) && match(key, p.Query)
		})
		if len(keys) > 0 {
			break
		}
	}
	if len(keys) == 0 {
		return e.reportMiss("lexicon", p.Query, "lexicons")
	}
	if p.JSON {
		return e.lexiconJSON(idx, p, keys)
	}
	fmt.Fprintf(e.Out, "LEXICON keys matching '%s' (%d keys, %d matching entries):\n", p.Query, len(keys), matches)
	for _, k := range keys[:min(lexiconKeysShown, len(keys))] {
		fmt.Fprintf(e.Out, "\n%s:%s\n", k.module, k.key)
		for _, r := range k.rows {
			fmt.Fprintf(e.Out, "  %-4s  %s  (%s!%s)\n", r.f.langLabel(), r.value, r.f.jar, r.f.path)
		}
	}
	if n := len(keys) - lexiconKeysShown; n > 0 {
		fmt.Fprintf(e.Out, "\n... and %d more keys (--json lists them all)\n", n)
	}
	e.printUnreadable(idx)
	return nil
}

func (e *Env) lexiconJSON(idx *index.Object, p LexiconParams, keys []*lexiconKey) error {
	list := make([]any, len(keys))
	for i, k := range keys {
		rows := make([]any, len(k.rows))
		for j, r := range k.rows {
			rows[j] = pyjson.Obj{{"language", r.f.language}, {"overrides", r.f.overrides}, {"jar", r.f.jar},
				{"path", r.f.path}, {"value", r.value}, {"matched", r.matched}}
		}
		list[i] = pyjson.Obj{{"module", k.module}, {"key", k.key}, {"rows", rows}}
	}
	fmt.Fprintln(e.Out, pyjson.DumpsAny(pyjson.Obj{{"query", p.Query}, {"text", ""}, {"module", p.Module}, {"lang", p.Lang},
		{"keys", list}, {"unreadable", anyList(objList(idx, "errors"))}}, pyjson.Indent2))
	return nil
}
