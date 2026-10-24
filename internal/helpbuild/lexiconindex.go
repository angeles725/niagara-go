package helpbuild

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// indexes/lexicon-index.json: every lexicon file of an install's modules/ (the
// files lexicon_extractor reads, isLexiconEntry) with all its entries:
//
//	{"jars": N, "jar_bytes": B,
//	 "files": [{"jar", "path", "module", "language", "overrides",
//	            "entries": [["key", "value"], ...]}],
//	 "errors": [{"jar", "path", "error"}]}
//
// module is the jar's module (moduleName of its META-INF/module.xml, or the
// jar name before its last "-" when the descriptor cannot be read). A file the
// descriptor declares as a <lexicon resource> is a translation: language is
// its language and overrides the base module whose lexicon it translates; any
// other file is a base lexicon (language and overrides ""). entries keep every
// key = value pair in file order, an empty value included, as ordered pairs
// (a key may repeat). A jar or member that cannot be read is an "errors" row
// and a warning, never a silent skip.
const (
	lexiconIndexName   = "lexicon-index.json"
	lexiconIndexScript = "lexicon_index_builder"
)

// lexiconIndex is the index being built.
type lexiconIndex struct {
	jarCensus
	rules         *Rules
	files         []pyjson.Value
	base, entries int
	modules       map[string]bool // the lexicon module of every file
	languages     map[string]bool
	keys          map[string]bool // lexicon module + "\x00" + key
}

// lexiconModule is the module whose lexicon a file belongs to: the module a
// translation overrides, else the jar's own.
func lexiconModule(module, overrides string) string {
	if overrides != "" {
		return overrides
	}
	return module
}

// descriptorOf is the module of a jar and its declared translations, keyed by
// resource; a descriptor that cannot be read is an errors row and the module
// is named after the jar.
func (x *lexiconIndex) descriptorOf(res *textResult, name string, members []jarMember) (string, map[string]modulexml.Lexicon) {
	module := moduleOf(name, nil)
	for _, m := range members {
		if m.name != modulexml.Member {
			continue
		}
		d, err := m.data, m.err
		var mod *modulexml.Module
		if err == nil {
			mod, err = modulexml.Parse(d)
		}
		if err != nil {
			x.fail(res, name, m.name, err)
			return module, nil
		}
		if mod.Part != "" {
			module = mod.Part
		}
		decl := map[string]modulexml.Lexicon{}
		for _, l := range mod.Lexicons {
			if _, dup := decl[l.Resource]; !dup {
				decl[l.Resource] = l
			}
		}
		return module, decl
	}
	return module, nil
}

// add indexes the lexicon files of one jar, or records why it cannot.
func (x *lexiconIndex) add(jar string, res *textResult) {
	name := x.count(jar)
	members, err := readMembers(jar, func(m string) bool { return m == modulexml.Member || isLexiconEntry(name, m, x.rules) })
	if err != nil {
		x.fail(res, name, "", err)
		return
	}
	if len(members) == 0 || len(members) == 1 && members[0].name == modulexml.Member {
		return
	}
	module, decl := x.descriptorOf(res, name, members)
	str := pyjson.String
	for _, m := range members {
		if m.name == modulexml.Member {
			continue
		}
		if m.err != nil {
			x.fail(res, name, m.name, m.err)
			continue
		}
		l := decl[m.name]
		lex := lexiconModule(module, l.Module)
		pairs := []pyjson.Value{}
		for _, e := range parseLexicon(m.data) {
			pairs = append(pairs, pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{str(e.key), str(e.value)}})
			x.keys[lex+"\x00"+e.key] = true
		}
		x.entries += len(pairs)
		x.modules[lex] = true
		if l.Language == "" {
			x.base++
		} else {
			x.languages[l.Language] = true
		}
		x.files = append(x.files, pyjson.ObjectOf(pyjson.Field{Key: "jar", Value: str(name)},
			pyjson.Field{Key: "path", Value: str(m.name)}, pyjson.Field{Key: "module", Value: str(module)},
			pyjson.Field{Key: "language", Value: str(l.Language)}, pyjson.Field{Key: "overrides", Value: str(l.Module)},
			pyjson.Field{Key: "entries", Value: pyjson.Value{Kind: pyjson.Array, A: pairs}}))
	}
}

// lexiconIndexDump writes the index on one line: its entries are most of an
// install's lexicon text, which two-space indentation would triple.
var lexiconIndexDump = pyjson.DumpOpts{Indent: -1, ItemSep: ",", KeySep: ":"}

// buildLexiconIndexFrom writes root/indexes/lexicon-index.json from the jars
// of installDir/modules.
func buildLexiconIndexFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	rules, err := Default()
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	x := &lexiconIndex{jarCensus: jarCensus{script: lexiconIndexScript}, rules: rules,
		modules: map[string]bool{}, languages: map[string]bool{}, keys: map[string]bool{}}
	for _, jar := range jars {
		x.add(jar, res)
	}
	arr := func(a []pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: a} }
	v := pyjson.ObjectOf(pyjson.Field{Key: "jars", Value: pyjson.IntValue(int64(x.jars))},
		pyjson.Field{Key: "jar_bytes", Value: pyjson.IntValue(int64(x.bytes))},
		pyjson.Field{Key: "files", Value: arr(x.files)}, pyjson.Field{Key: "errors", Value: arr(x.errors)})
	out := pycompat.Join(root, "indexes", lexiconIndexName)
	if err := pyfs.WriteJSON(out, v, lexiconIndexDump); err != nil {
		return nil, err
	}
	res.Files = len(x.files)
	// A key is translated in many files: entries are rows, distinct module keys the module + key pairs.
	res.Log = append(res.Log, fmt.Sprintf("%s: %d lexicon files (%d base, %d translations) of %d modules in %d languages,"+
		" %d entries (%d distinct module keys) from %d jars (%d errors) -> %s",
		lexiconIndexName, res.Files, x.base, res.Files-x.base, len(x.modules), len(x.languages),
		x.entries, len(x.keys), x.jars, len(x.errors), out))
	return res, nil
}

// buildLexiconIndex is the rebuild-all step: lexicon-index.json from the
// modules/ of the install registered for root.
var buildLexiconIndex = fromInstall(lexiconIndexName, buildLexiconIndexFrom)
