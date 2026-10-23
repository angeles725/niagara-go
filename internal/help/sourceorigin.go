package help

// Provenance of indexed Java source: helpbuild extracts docSource-doc.jar
// and the vendor source module jars ship (source/<jar stem>/); a record of
// source-index.json taken from a module jar carries "origin": "module-jar
// <jar>", a docSource record none. docSource wins a shared simple name, so
// a module-jar copy is the "Name (module)" alternate. The origin is shown
// only where a module-jar record is involved, so a docSource-only corpus
// prints what it always did.

import "github.com/angeles725/niagara-go/internal/index"

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
