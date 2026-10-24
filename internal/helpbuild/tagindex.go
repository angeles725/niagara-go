package helpbuild

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/tagdict"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// indexes/tag-index.json: the tag dictionaries of an install's modules/ (the
// Haystack files of haystack*.jar, the dictionary bogs of tagdictionary*.jar):
//
//	{"jars": N, "jar_bytes": B,
//	 "files": [{"jar", "path", "kind", "records"}],
//	 "tags": [...], "protos": [...], "groups": [...], "relations": [...],
//	 "rules": [...], "migrations": [...],
//	 "errors": [{"jar", "path", "error"}]}
//
// Each record row is tagdict's (File.Append) and names the jar!path it came
// from. A file (or jar) that cannot be read is an "errors" row and a warning,
// never a silent skip; jars and jar_bytes count every jar of modules/, which
// freshness compares with the registered install.
const (
	tagIndexName   = "tag-index.json"
	tagIndexScript = "tag_index_builder"
)

// tagSections are the record sections of tag-index.json, in file order.
var tagSections = []string{"tags", "protos", "groups", "relations", "rules", "migrations"}

// isTagDictionary reports whether member of the jar named jar is a tag
// dictionary file: a .json or .csv of a haystack jar, a .bog of a
// tagdictionary jar.
func isTagDictionary(jar, member string) bool {
	jar, m := textfold.Lower(jar), textfold.Lower(member)
	switch {
	case strings.HasPrefix(jar, "haystack"):
		return strings.HasSuffix(m, ".json") || strings.HasSuffix(m, ".csv")
	case strings.HasPrefix(jar, "tagdictionary"):
		return strings.HasSuffix(m, ".bog")
	}
	return false
}

// isTagDictionaryJar reports whether a jar may hold tag dictionary files.
func isTagDictionaryJar(jar string) bool {
	jar = textfold.Lower(jar)
	return strings.HasPrefix(jar, "haystack") || strings.HasPrefix(jar, "tagdictionary")
}

// tagIndex is the index being built.
type tagIndex struct {
	jarCensus
	files    []pyjson.Value
	sections map[string][]pyjson.Value
	distinct map[string]bool // dict:name of every tag row
}

// add indexes the dictionary files of one jar, or records why it cannot.
func (x *tagIndex) add(jar string, res *textResult) {
	name := x.count(jar)
	if !isTagDictionaryJar(name) {
		return
	}
	members, err := readMembers(jar, func(m string) bool { return isTagDictionary(name, m) })
	if err != nil {
		x.fail(res, name, "", err)
		return
	}
	for _, m := range members {
		if m.err != nil {
			x.fail(res, name, m.name, m.err)
			continue
		}
		f, err := tagdict.Read(m.name, m.data)
		if err != nil {
			x.fail(res, name, m.name, err)
			continue
		}
		x.files = append(x.files, pyjson.ObjectOf(pyjson.Field{Key: "jar", Value: pyjson.String(name)},
			pyjson.Field{Key: "path", Value: pyjson.String(m.name)}, pyjson.Field{Key: "kind", Value: pyjson.String(f.Kind)},
			pyjson.Field{Key: "records", Value: pyjson.IntValue(int64(f.Records()))}))
		for _, t := range f.Tags {
			x.distinct[t.Dict+":"+t.Name] = true
		}
		f.Append(name+"!"+m.name, func(section string, v pyjson.Value) {
			x.sections[section] = append(x.sections[section], v)
		})
	}
}

// buildTagIndexFrom writes root/indexes/tag-index.json from the jars of
// installDir/modules.
func buildTagIndexFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	x := &tagIndex{jarCensus: jarCensus{script: tagIndexScript}, sections: map[string][]pyjson.Value{}, distinct: map[string]bool{}}
	for _, jar := range jars {
		x.add(jar, res)
	}
	arr := func(a []pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: a} }
	v := pyjson.ObjectOf(pyjson.Field{Key: "jars", Value: pyjson.IntValue(int64(x.jars))},
		pyjson.Field{Key: "jar_bytes", Value: pyjson.IntValue(int64(x.bytes))},
		pyjson.Field{Key: "files", Value: arr(x.files)})
	counts := make([]any, 0, len(tagSections))
	for _, s := range tagSections {
		v.Set(s, arr(x.sections[s]))
		counts = append(counts, len(x.sections[s]))
	}
	v.Set("errors", arr(x.errors))
	out := pycompat.Join(root, "indexes", tagIndexName)
	if err := pyfs.WriteJSON(out, v, pyfs.DumpIndent2Unicode); err != nil {
		return nil, err
	}
	res.Files = len(x.files)
	// A tag defined in several files has one row per file: the log names both counts.
	res.Log = append(res.Log, fmt.Sprintf("%s: %d tag rows (%d distinct tags), %d protos, %d groups, %d relations, %d rules, %d migrations"+
		" from %d files in %d jars (%d errors) -> %s",
		append(append([]any{tagIndexName, counts[0], len(x.distinct)}, counts[1:]...), res.Files, x.jars, len(x.errors), out)...))
	return res, nil
}

// buildTagIndex is the rebuild-all step: tag-index.json from the modules/ of
// the install registered for root.
var buildTagIndex = fromInstall(tagIndexName, buildTagIndexFrom)
