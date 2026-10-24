// Package tagdict reads the tag dictionaries Niagara ships into the records
// nh keeps in indexes/tag-index.json: the Haystack dictionary files of
// haystack-rt (Haystack 4 defs.json and protos.json, the Niagara implied-tag
// rules HS4toN4.json, and the CSV tables tags.csv, smartRefsImport.csv,
// equip.csv and haystack3MigrationConfig.csv) and the Niagara dictionary of
// tagdictionary-rt (bog/Niagara.bog). It is a leaf: it reads bytes, not jars.
//
// A file it cannot recognize or that lacks what its kind requires is an
// error, never an empty dictionary.
package tagdict

import (
	"errors"
	"path"
	"strings"
)

// Haystack is the namespace of the Haystack dictionary (HaystackTagDictionary).
const Haystack = "hs"

// File is what one dictionary file holds. Kind names the file's format:
// "defs", "protos", "rules", "tags", "groups", "migrations" or "niagara".
type File struct {
	Kind       string
	Tags       []Tag
	Protos     []string
	Groups     []Group
	Relations  []Relation
	Rules      []Rule
	Migrations []Migration
}

// Records is how many records f holds.
func (f *File) Records() int {
	return len(f.Tags) + len(f.Protos) + len(f.Groups) + len(f.Relations) + len(f.Rules) + len(f.Migrations)
}

// Tag is one tag definition. Kind is the value kind ("Marker", "Str", ...)
// when the source says it; Is and Supertypes are a Haystack 4 def's direct
// and transitive supertypes; Implied and Validity are the rule cells a CSV
// table or the Niagara dictionary sets.
type Tag struct {
	Dict, Name, Kind, Doc string
	Is, Supertypes, TagOn []string
	Of, Lib, SmartType    string
	Units                 string
	Implied, Validity     []Field
}

// Field is one named cell of a rule.
type Field struct{ Name, Value string }

// Group is a tag group: a name standing for a set of tags.
type Group struct {
	Dict, Name string
	Tags       []string
	Validity   []Field
}

// Relation is a relation definition.
type Relation struct{ Dict, Name, Type string }

// Rule applies tags, tag groups and relations to the objects its condition
// accepts.
type Rule struct {
	Dict, Name, Condition string
	Tags, Relations       []RuleTag
	Groups                []string
}

// RuleTag is a tag or relation a rule applies, with its implementing type
// and its own validity when it has them.
type RuleTag struct{ Name, Type, Validity string }

// Migration maps a Haystack 3 name to its Haystack 4 names.
type Migration struct {
	From, FromType, Query string
	To                    []string
	ToType                string
}

// Read reads the dictionary file name (a jar member path) holding data.
func Read(name string, data []byte) (*File, error) {
	switch strings.ToLower(path.Ext(name)) {
	case ".json":
		return readJSON(data)
	case ".csv":
		return readCSV(data)
	case ".bog":
		return readNiagara(data)
	}
	return nil, errors.New("not a tag dictionary file (.json, .csv or .bog)")
}
