package helpbuild

import (
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/indexes"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Vendor Java source of module jars (moduleSource) lands under
// source/<jar stem>/, next to docSource's folders, and feeds the same
// source, slots and xref indexes. docSource wins: a module-jar source whose
// path a doc jar already maps is never written, checked or recorded (the
// doc jars come first in listDocJars). Which files came from a module jar
// is recorded in source/<indexes.SourceOriginsFile>, so the source index
// can tell them apart; the record is made from the jars' mapping, not from
// what a pass wrote, so extract and fill-missing record the same files.

// sourceOwners follows, over the jars of one extraction pass in
// listDocJars order, the jar that maps each corpus path, and the
// module-jar sources the pass takes.
type sourceOwners struct {
	owner  map[string]string // corpus path -> jar
	origin map[string]string // path under source/ -> module jar
}

func newSourceOwners() *sourceOwners {
	return &sourceOwners{owner: map[string]string{}, origin: map[string]string{}}
}

// isModuleSourcePath: rel is where a module jar's Java source maps.
func isModuleSourcePath(jarName, rel string) bool {
	return isModuleJar(jarName) && strings.HasPrefix(rel, "source/")
}

// take records that jarName maps rel; prev is the jar that mapped it
// before (""). take is false for a module-jar source an earlier jar
// already maps: the pass leaves that path to it.
func (o *sourceOwners) take(jarName, rel string) (prev string, take bool) {
	prev = o.owner[rel]
	if prev != "" && isModuleSourcePath(jarName, rel) {
		return prev, false
	}
	o.owner[rel] = jarName
	if isModuleSourcePath(jarName, rel) {
		o.origin[strings.TrimPrefix(rel, "source/")] = jarName
	}
	return prev, true
}

// write saves the module-jar sources taken as source/<SourceOriginsFile>
// (sorted keys, atomic). A pass that took none writes nothing.
func (o *sourceOwners) write(root string) error {
	if len(o.origin) == 0 {
		return nil
	}
	keys := make([]string, 0, len(o.origin))
	for k := range o.origin {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m := pyjson.NewObject()
	for _, k := range keys {
		m.Set(k, pyjson.String(o.origin[k]))
	}
	return pyfs.WriteJSON(pycompat.Join(root, "source", indexes.SourceOriginsFile), m, pyfs.DumpIndent2Unicode)
}
