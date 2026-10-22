package indexes

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// xref_builder.py: indexes/xref-index.json is the import graph of source/
// and devguide-rc/: per class (the file's top-level type) the Niagara
// classes it imports ("uses"), the classes importing it ("used_by") and
// its package.

// xrefImportRe is extract_imports' import statement.
var xrefImportRe = pyre.MustCompile(`import\s+(?:static\s+)?([\w.]+)\s*;`, 0)

// errEmptyImportName is the IndexError extract_imports raises for an
// import whose name ends with "." ("".isupper() is never reached:
// parts[-1][0] fails first). Python does not catch it.
var errEmptyImportName = errors.New("IndexError: string index out of range")

// xrefFile is one extract_imports result.
type xrefFile struct {
	class, pkg string
	imports    []string
}

// extractImports is extract_imports: the stripped lines up to the first
// type declaration give the package (the last "package " line), the
// imported simple names that start with an upper-case letter, and the
// declared name; a file without one (or that cannot be read) is named
// after its file.
func extractImports(path string) (xrefFile, error) {
	var f xrefFile
	var err error
	if text, ok := readJavaText(path); ok {
		javaFileLines(text, func(line string) bool {
			line = pycompat.Strip(line)
			switch {
			case strings.HasPrefix(line, "package "):
				f.pkg, _ = javaPackage(line)
			case strings.HasPrefix(line, "import "):
				m := xrefImportRe.MatchRunes([]rune(line), 0)
				if m == nil {
					return true
				}
				fqn := m.Group(1)
				last := fqn[strings.LastIndexByte(fqn, '.')+1:]
				if last == "" {
					err = errEmptyImportName
					return false
				}
				first, _ := firstRune(last)
				if pycompat.IsUpper(first) {
					f.imports = append(f.imports, last)
				}
			default:
				f.class = javaDeclaration(line)
				return f.class == ""
			}
			return true
		})
	}
	if err != nil {
		return f, err
	}
	if f.class == "" {
		f.class = strings.ReplaceAll(filepath.Base(path), ".java", "")
	}
	return f, nil
}

// firstRune is s[0] as a str.
func firstRune(s string) (string, bool) {
	for _, r := range s {
		return string(r), true
	}
	return "", false
}

// BuildXrefIndex is xref_builder.build_xref(baseDir) (the script's
// __main__ with baseDir as its argument): every file is read first, then
// only the imports of Niagara types (B*, Niagara*, or a type the corpus
// declares) are kept, so the result does not depend on the walk order.
// err != nil is where Python raises (source/ that cannot be listed, an
// import ending with ".", a failed write: traceback, exit 1).
func BuildXrefIndex(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	roots, err := iterJavaRoots(baseDir, res)
	if err != nil {
		return res, err
	}
	var parsed []xrefFile
	for _, root := range roots {
		pyfs.Walk(root.dir, func(dir string, dirs, files []string) []string {
			sort.Strings(dirs)
			sort.Strings(files)
			for _, name := range files {
				if err != nil || !strings.HasSuffix(name, ".java") {
					continue
				}
				var f xrefFile
				if f, err = extractImports(pycompat.Join(dir, name)); err == nil {
					parsed = append(parsed, f)
				}
			}
			if err != nil {
				return nil
			}
			return dirs
		})
		if err != nil {
			return res, err
		}
	}
	known := map[string]bool{}
	for _, f := range parsed {
		known[f.class] = true
	}
	uses, usedBy := map[string]map[string]bool{}, map[string]map[string]bool{}
	packages := map[string]string{}
	for _, f := range parsed {
		if f.pkg != "" {
			packages[f.class] = f.pkg
		} else if _, ok := packages[f.class]; !ok {
			packages[f.class] = ""
		}
		if uses[f.class] == nil {
			uses[f.class] = map[string]bool{}
		}
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, "B") || strings.HasPrefix(imp, "Niagara") || known[imp] {
				uses[f.class][imp] = true
				if usedBy[imp] == nil {
					usedBy[imp] = map[string]bool{}
				}
				usedBy[imp][f.class] = true
			}
		}
	}
	all := map[string]bool{}
	for _, m := range []map[string]map[string]bool{uses, usedBy} {
		for c := range m {
			all[c] = true
		}
	}
	for c := range packages {
		all[c] = true
	}
	out := pyjson.NewObject()
	edges := 0
	for _, cls := range slices.Sorted(maps.Keys(all)) {
		entry := pyjson.NewObject()
		if u := sortedOthers(uses[cls], cls); len(u) > 0 {
			entry.Set("uses", pyjson.StringList(u))
			edges += len(u)
		}
		if u := sortedOthers(usedBy[cls], cls); len(u) > 0 {
			entry.Set("used_by", pyjson.StringList(u))
		}
		if pkg, ok := packages[cls]; ok {
			entry.Set("package", pyjson.String(pkg))
		}
		if len(entry.Keys) > 0 {
			out.Set(cls, entry)
		}
	}
	if err := pyfs.WriteJSON(pycompat.Join(baseDir, "indexes", "xref-index.json"), out, pyfs.DumpIndent2Unicode); err != nil {
		return res, err
	}
	res.out("Built xref-index.json: %d classes, %d dependency edges from %d source files", len(out.Keys), edges, len(parsed))
	return res, nil
}

// sortedOthers is sorted(set - {self}).
func sortedOthers(set map[string]bool, self string) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(set)) {
		if k != self {
			out = append(out, k)
		}
	}
	return out
}
