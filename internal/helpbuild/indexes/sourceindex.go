package indexes

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// source_indexer.py: indexes/source-index.json maps each .java file under
// source/<module>/ and devguide-rc/ to its module, path (relative to
// source/), package and line count, keyed by the file name. A file taken
// from a module jar (SourceOriginsFile) also has "origin": "module-jar
// <jar>" and is indexed after every docSource file (sourceorigin.go).

// OriginModuleJar starts the origin of a source-index record extracted
// from a module jar; a docSource record has no origin.
const OriginModuleJar = "module-jar "

// sourceEntry is one source-index.json value.
type sourceEntry struct {
	module, path, pkg string
	lines             int
	jar               string // the module jar it was extracted from, "" for docSource
}

// BuildSourceIndex is source_indexer.index_source(baseDir) (the script's
// __main__ with baseDir as its argument). The folders are walked sorted,
// the files of each folder in the order the file system lists them (as
// os.walk does when only dirs is sorted), which fixes the key order and
// which of two same-name files gets the plain key. A file that cannot be
// opened is skipped. err != nil is where Python raises (source/ that
// cannot be listed, a failed write: traceback, exit 1).
func BuildSourceIndex(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	roots, err := iterJavaRoots(baseDir, res)
	if err != nil {
		return res, err
	}
	sourceDir, err := absSourceDir(baseDir)
	if err != nil {
		return res, err
	}
	origins, err := readSourceOrigins(baseDir)
	if err != nil {
		return res, err
	}
	index := map[string]sourceEntry{}
	var order []string
	totalLines := 0
	add := func(module, name, path, jar string) {
		text, ok := readJavaText(path)
		if !ok {
			return
		}
		pkg, count := "", 0
		javaFileLines(text, func(line string) bool {
			count++
			if pkg == "" {
				pkg, _ = javaPackage(line)
			}
			return true
		})
		totalLines += count
		key := strings.TrimSuffix(name, ".java")
		if _, dup := index[key]; dup {
			key += " (" + module + ")"
		}
		if _, seen := index[key]; !seen {
			order = append(order, key)
		}
		index[key] = sourceEntry{module, relToSource(sourceDir, path), pkg, count, jar}
	}
	type later struct{ module, name, path, jar string }
	var fromJars []later // module-jar sources, indexed after docSource
	for _, root := range roots {
		pycompat.WalkWith(root.dir, pycompat.WalkOptions{Unsorted: true}, func(dir string, dirs, files []string) []string {
			sort.Strings(dirs)
			for _, name := range files {
				if !strings.HasSuffix(name, ".java") {
					continue
				}
				path := pycompat.Join(dir, name)
				if jar := origins.jar(sourceDir, path); jar != "" {
					fromJars = append(fromJars, later{root.module, name, path, jar})
					continue
				}
				add(root.module, name, path, "")
			}
			return dirs
		})
	}
	for _, f := range fromJars {
		add(f.module, f.name, f.path, f.jar)
	}
	out := pyjson.NewObject()
	for _, key := range order {
		e := index[key]
		rec := pyjson.ObjectOf(pyjson.Field{Key: "module", Value: pyjson.String(e.module)},
			pyjson.Field{Key: "path", Value: pyjson.String(e.path)},
			pyjson.Field{Key: "package", Value: pyjson.String(e.pkg)},
			pyjson.Field{Key: "lines", Value: pyjson.IntValue(int64(e.lines))})
		if e.jar != "" {
			rec.Set("origin", pyjson.String(OriginModuleJar+e.jar))
		}
		out.Set(key, rec)
	}
	if err := pyfs.WriteJSON(pycompat.Join(baseDir, "indexes", "source-index.json"), out, pyfs.DumpIndent2Unicode); err != nil {
		return res, err
	}
	res.out("Built source-index.json: %d classes, %d total lines from source/", len(order), totalLines)
	return res, nil
}

// relToSource is os.path.relpath(path, source_dir).replace("\\", "/"):
// sourceDir is already absolute, both are cleaned lexically.
func relToSource(sourceDir, path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		if rel, err := filepath.Rel(sourceDir, abs); err == nil {
			path = rel
		}
	}
	return strings.ReplaceAll(path, "\\", "/")
}
