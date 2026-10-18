package fts

// The text sources the full-text search covers (text_sources.py): one place
// lists the labels, their directories and file extensions, so the search,
// the fingerprint and the honest-zero scope agree on what "full text" is.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// source is one text_sources.SOURCES row.
type source struct {
	label string
	exts  []string
}

// sources are text_sources.SOURCES, in index (doc id) order.
var sources = []source{
	{"bajadoc-clean", []string{".txt"}},
	{"devguide-clean", []string{".txt"}},
	{"guides-clean", []string{".txt"}},
	{"docs-text", []string{".txt"}},
	{"course-text", []string{".txt"}},
	{"source", []string{".java"}},
	{"devguide-rc", []string{".java", ".txt"}},
	{"devguide-examples", []string{".js", ".hbs", ".px", ".html", ".java"}}, // doc_types.EXAMPLE_EXT
}

// Labels are text_sources.LABELS.
var Labels = func() []string {
	out := make([]string, len(sources))
	for i, s := range sources {
		out[i] = s.label
	}
	return out
}()

// Shared are text_sources.SHARED: the sources a version without its own copy
// borrows from another registered version, with the note find, pdf and the
// scope report print ({} is the owner version).
var Shared = map[string]string{
	"docs-text":   "the PDF manuals of Niagara %s (this version ships none)",
	"course-text": "the developer course bound to Niagara %s",
}

// SourceDir is one (label, absolute directory) of SourceDirs.
type SourceDir struct{ Label, Dir string }

// SourceDirs is text_sources.source_dirs: every source present in baseDir,
// docs-text and course-text possibly borrowed from another version.
func SourceDirs(baseDir string) []SourceDir {
	var out []SourceDir
	for _, label := range Labels {
		root := baseDir
		if _, ok := Shared[label]; ok {
			root, _ = SharedRoot(baseDir, label)
		}
		dir := abs(filepath.Join(root, label))
		if pycompat.IsDir(dir) {
			out = append(out, SourceDir{label, dir})
		}
	}
	return out
}

// DirOf is dict(source_dirs(baseDir)).get(label): "" when absent.
func DirOf(dirs []SourceDir, label string) string {
	for _, d := range dirs {
		if d.Label == label {
			return d.Dir
		}
	}
	return ""
}

// SharedRoot is text_sources.shared_root: the data root whose <label>/ this
// version searches and, when it belongs to another registered version, that
// version id. Only a registered data root borrows; the registry's default
// is tried first, then the ids in sorted order.
func SharedRoot(baseDir, label string) (root, owner string) {
	base := abs(baseDir)
	if hasText(filepath.Join(base, label)) || identify(base) == "" {
		return base, ""
	}
	for _, repo := range []string{base, filepath.Dir(filepath.Dir(base))} {
		reg, err := index.LoadRegistry(repo)
		if err != nil || reg == nil || reg.Versions == nil || reg.Versions.Len() == 0 {
			continue
		}
		ids := append([]string(nil), reg.Versions.Keys()...)
		sort.Strings(ids)
		if reg.Versions.Has(reg.Default) {
			ids = append([]string{reg.Default}, ids...)
		}
		for _, vid := range ids {
			r, err := index.DataRoot(repo, vid)
			if err == nil && hasText(filepath.Join(r, label)) {
				return r, vid
			}
		}
	}
	return base, ""
}

// DocsTextRoot is text_sources.docs_text_root: SharedRoot of the PDF manuals.
func DocsTextRoot(baseDir string) (root, owner string) { return SharedRoot(baseDir, "docs-text") }

// identify is versions.identify: the registered version whose data root is
// baseDir (registry at baseDir or two levels up), "" when none.
func identify(baseDir string) string {
	target := abs(baseDir)
	for _, repo := range []string{target, filepath.Dir(filepath.Dir(target))} {
		reg, err := index.LoadRegistry(repo)
		if err != nil || reg == nil {
			continue
		}
		for _, vid := range reg.Versions.Keys() {
			if r, err := index.DataRoot(repo, vid); err == nil && r == target {
				return vid
			}
		}
	}
	return ""
}

// hasText is text_sources._has_text: dir holds a .txt file at its top level.
func hasText(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			return true
		}
	}
	return false
}

// skipExample is doc_types.example_skip_reason != None: the bundled
// third-party library (top-level ext/) and minified builds are not indexed.
func skipExample(rel string) bool {
	first, _, _ := strings.Cut(rel, "/")
	return first == "ext" || strings.HasSuffix(strings.ToLower(rel), ".min.js")
}

// IterFiles is text_sources.iter_files: the indexed files of one source in
// os.walk order (a folder's files sorted, then its sub-folders sorted, depth
// first), as absolute paths.
func IterFiles(label, dir string) []string {
	var exts []string
	for _, s := range sources {
		if s.label == label {
			exts = s.exts
		}
	}
	var out []string
	walkFiles(dir, func(path string) {
		name := filepath.Base(path)
		ok := false
		for _, e := range exts {
			ok = ok || strings.HasSuffix(name, e)
		}
		if ok && !(label == "devguide-examples" && skipExample(rel(dir, path))) {
			out = append(out, path)
		}
	})
	return out
}

// walkFiles calls fn for every file os.walk(dir) lists, in its top-down
// order with sorted names: an entry whose stat says directory is a folder
// (a symbolic link to one is listed but not entered), anything else a file.
// An unreadable folder lists nothing, like os.walk without onerror.
func walkFiles(dir string, fn func(path string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var dirs []string
	for _, e := range entries { // os.ReadDir sorts by name (byte order = code-point order)
		path := filepath.Join(dir, e.Name())
		isLink := e.Type()&os.ModeSymlink != 0
		switch {
		case isLink && pycompat.IsDir(path):
			// listed among the folders, never walked (followlinks=False)
		case e.IsDir():
			dirs = append(dirs, path)
		default:
			fn(path)
		}
	}
	for _, d := range dirs {
		walkFiles(d, fn)
	}
}

// CountFiles is sum(len(files) for os.walk(dir)): every file, indexed or not.
func CountFiles(dir string) int {
	n := 0
	walkFiles(dir, func(string) { n++ })
	return n
}

// rel is os.path.relpath(path, dir) with "/" separators for a path under dir.
func rel(dir, path string) string {
	r, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

// abs is os.path.abspath.
func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
