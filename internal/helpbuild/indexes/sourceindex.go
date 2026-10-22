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
// source/), package and line count, keyed by the file name.

// sourceEntry is one source-index.json value.
type sourceEntry struct {
	module, path, pkg string
	lines             int
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
	sourceDir, err := filepath.Abs(pycompat.Join(baseDir, "source"))
	if err != nil {
		return res, err
	}
	index := map[string]sourceEntry{}
	var order []string
	totalLines := 0
	for _, root := range roots {
		pycompat.WalkWith(root.dir, pycompat.WalkOptions{Unsorted: true}, func(dir string, dirs, files []string) []string {
			sort.Strings(dirs)
			for _, name := range files {
				if !strings.HasSuffix(name, ".java") {
					continue
				}
				path := pycompat.Join(dir, name)
				text, ok := readJavaText(path)
				if !ok {
					continue
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
					key += " (" + root.module + ")"
				}
				if _, seen := index[key]; !seen {
					order = append(order, key)
				}
				index[key] = sourceEntry{root.module, relToSource(sourceDir, path), pkg, count}
			}
			return dirs
		})
	}
	out := pyjson.NewObject()
	for _, key := range order {
		e := index[key]
		out.Set(key, pyjson.ObjectOf(pyjson.Field{Key: "module", Value: pyjson.String(e.module)},
			pyjson.Field{Key: "path", Value: pyjson.String(e.path)},
			pyjson.Field{Key: "package", Value: pyjson.String(e.pkg)},
			pyjson.Field{Key: "lines", Value: pyjson.IntValue(int64(e.lines))}))
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
