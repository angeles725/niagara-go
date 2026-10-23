package help

// Provenance of indexed Java source: helpbuild extracts docSource-doc.jar
// and the vendor source module jars ship (source/<jar stem>/); a record of
// source-index.json taken from a module jar carries "origin": "module-jar
// <jar>", a docSource record none. docSource wins a shared simple name, so
// a module-jar copy is the "Name (module)" alternate. The origin is shown
// only where a module-jar record is involved, so a docSource-only corpus
// prints what it always did.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// sourceOrigin is the origin of a source-index record: its "origin", or
// "docSource".
func sourceOrigin(data *index.Object) string {
	if o := getStr(data, "origin", ""); o != "" {
		return o
	}
	return "docSource"
}

// fromModuleJar: the record was extracted from a module jar.
func fromModuleJar(data *index.Object) bool { return getStr(data, "origin", "") != "" }

// anyFromModuleJar: some match came from a module jar.
func anyFromModuleJar(ms []sourceMatch) bool {
	for _, m := range ms {
		if fromModuleJar(m.data) {
			return true
		}
	}
	return false
}

// classFromSource is class's fallback for a name class-index.json does not
// hold (no javadoc): the first source-index record of that simple name
// (and package, for a qualified name), shown with its origin. ok is false
// when the source index has none either.
func (e *Env) classFromSource(name string) (ok bool, err error) {
	idx, err := e.loadIndex("source-index.json", false)
	if err != nil {
		return false, err
	}
	pkg, simple := "", name
	if i := strings.LastIndex(name, "."); i >= 0 {
		pkg, simple = name[:i], name[i+1:]
	}
	for _, m := range sourceMatches(idx, simple) {
		if pkg != "" && getStr(m.data, "package", "") != pkg {
			continue
		}
		fmt.Fprintf(e.Out, "CLASS: %s\nPACKAGE: %s\nMODULE: %s\n", simple, pyStr(get(m.data, "package", nil)),
			pyStr(get(m.data, "module", nil)))
		fmt.Fprintln(e.Out, "No javadoc (.bajadoc) for this class; its Java source is indexed:")
		fmt.Fprintf(e.Out, "SOURCE: %s (%s lines)\nORIGIN: %s\n", corpusPath(m.data), pyStr(get(m.data, "lines", nil)),
			sourceOrigin(m.data))
		key := m.key
		if strings.Contains(key, " ") {
			key = "'" + key + "'"
		}
		fmt.Fprintf(e.Out, "Run: nh source %s --code\n", key)
		return true, nil
	}
	return false, nil
}

// printModuleJarAlternates lists, under a class shown from its javadoc,
// the Java source module jars ship under the same simple name.
func (e *Env) printModuleJarAlternates(simple string) error {
	idx, err := e.loadIndex("source-index.json", false)
	if err != nil {
		return err
	}
	for _, m := range sourceMatches(idx, simple) {
		if fromModuleJar(m.data) {
			fmt.Fprintf(e.Out, "ALSO: Java source %s [%s]\n", m.key, sourceOrigin(m.data))
		}
	}
	return nil
}
