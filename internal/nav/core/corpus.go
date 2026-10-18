package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The organized/ corpus (module_nav_lib/corpus.py):
//
//	<parent>/module-navigator/        the checkout
//	<parent>/organized/<module>/<submodule>/{vineflower,extracted,<submodule>.jar,...}

// ResolveOrganized returns the corpus root of the runtime's checkout
// (OrganizedDir) when it exists, else "" and the path tried.
func (r *Runtime) ResolveOrganized() (path, tried string) { return resolveDir(r.OrganizedDir()) }

// resolveDir is (p, "") when p is a directory, else ("", p).
func resolveDir(p string) (path, tried string) {
	if pycompat.IsDir(p) {
		return p, ""
	}
	return "", p
}

// SourceRoot is corpus.source_root for commands that read sources: the corpus
// root, or "" after an exit-2 error. meta is the _meta block of the index in
// use: when it was built from another corpus that still exists, the hint names it.
func (r *Runtime) SourceRoot(meta *index.Object) string {
	path, message, hint := FindSourceRootAt(r.OrganizedDir(), meta)
	if path == "" {
		r.Error(message, hint)
	}
	return path
}

// FindSourceRootAt is SourceRoot without a runtime, with the corpus location
// given (organized, which may not exist): the corpus root, or "" with the
// error message and hint to report.
func FindSourceRootAt(organized string, meta *index.Object) (path, message, hint string) {
	path, tried := resolveDir(organized)
	if path != "" {
		return path, "", ""
	}
	hint = "Set NAV_CORPUS_BASE to the organized/ directory."
	if rec := metaSource(meta); rec != "" && pycompat.IsDir(rec) && realpath(rec) != realpath(tried) {
		hint = "The index was built from " + rec + ", which exists: export NAV_CORPUS_BASE=" + rec
	}
	return "", "corpus not found: " + tried, hint
}

// submoduleMarkers: any of these inside organized/<module>/<dir>/ makes <dir> a submodule.
var submoduleMarkers = []string{"vineflower", "decompiled", "extracted", filepath.Join("pipeline", "fase1-recon.json")}

// typeSuffixes are the submodule name suffixes that give its type.
var typeSuffixes = []string{"rt", "wb", "ux", "doc", "se"}

// IsSubmodule is corpus.is_submodule: path holds pipeline output or its own JAR.
func IsSubmodule(path string) bool {
	if pycompat.IsFile(filepath.Join(path, filepath.Base(path)+".jar")) {
		return true
	}
	for _, m := range submoduleMarkers {
		if _, err := os.Stat(filepath.Join(path, m)); err == nil {
			return true
		}
	}
	return false
}

// SubmoduleType is corpus.submodule_type: "rt", "wb", "ux", "doc", "se", or ""
// for the standalone submodule and for an unknown suffix.
func SubmoduleType(module, submodule string) string {
	if submodule == module {
		return ""
	}
	for _, s := range typeSuffixes {
		if strings.HasSuffix(submodule, "-"+s) {
			return s
		}
	}
	return ""
}

// SubmoduleKind is corpus.submodule_kind: SubmoduleType with "standalone" for
// <module>/<module> and "other" for an unknown suffix.
func SubmoduleKind(module, submodule string) string {
	if submodule == module {
		return "standalone"
	}
	if t := SubmoduleType(module, submodule); t != "" {
		return t
	}
	return "other"
}

// Submodule is one organized/<module>/<submodule>/ directory.
type Submodule struct{ Module, Name, Path string }

// IterSubmodules is corpus.iter_submodules: every submodule, sorted. The other
// directories (docs/, stray META-INF/) are appended to skipped, relative to
// organized/, when skipped is not nil.
func IterSubmodules(organized string, skipped *[]string) []Submodule {
	var out []Submodule
	for _, module := range sortedNames(organized) {
		mpath := filepath.Join(organized, module)
		if !pycompat.IsDir(mpath) {
			continue
		}
		for _, sub := range sortedNames(mpath) {
			spath := filepath.Join(mpath, sub)
			if !pycompat.IsDir(spath) {
				continue
			}
			if IsSubmodule(spath) {
				out = append(out, Submodule{module, sub, spath})
			} else if skipped != nil {
				*skipped = append(*skipped, module+"/"+sub)
			}
		}
	}
	return out
}

// JavaFile is one vineflower source: its submodule, absolute path and path
// relative to organized/ with forward slashes.
type JavaFile struct{ Sub, Path, Rel string }

// IterJava is corpus.iter_java: every vineflower .java file in a stable order
// (os.walk top-down with sorted directories and files; symlinked directories
// are listed but not entered).
func IterJava(organized string) []JavaFile {
	var out []JavaFile
	WalkJava(organized, nil, func(f JavaFile) bool {
		out = append(out, f)
		return true
	})
	return out
}

// WalkJava is the lazy corpus.iter_java: fn receives every vineflower .java
// file of the submodules keep accepts (all of them when keep is nil), in
// IterJava order, and ends the walk early by returning false. Skipping a
// submodule as a whole yields the same files, in the same order, as walking it
// and filtering each file by its submodule.
func WalkJava(organized string, keep func(Submodule) bool, fn func(JavaFile) bool) {
	var walk func(sub, dir string) bool
	walk = func(sub, dir string) bool {
		// os.walk over scandir: an entry is a directory when it is one or links
		// to one (the entry type saves a stat for everything but symlinks); a
		// symlinked directory is listed but not entered.
		var dirs, files []string
		links := map[string]bool{}
		for _, e := range sortedEntries(dir) {
			name := e.Name()
			switch {
			case e.IsDir():
				dirs = append(dirs, name)
			case e.Type()&os.ModeSymlink != 0 && pycompat.IsDir(filepath.Join(dir, name)):
				dirs = append(dirs, name)
				links[name] = true
			default:
				files = append(files, name)
			}
		}
		for _, name := range files {
			if strings.HasSuffix(name, ".java") {
				p := filepath.Join(dir, name)
				rel, err := filepath.Rel(organized, p)
				if err != nil {
					rel = p
				}
				if !fn(JavaFile{sub, p, strings.ReplaceAll(filepath.ToSlash(rel), "\\", "/")}) {
					return false
				}
			}
		}
		for _, name := range dirs {
			if links[name] {
				continue
			}
			if !walk(sub, filepath.Join(dir, name)) {
				return false
			}
		}
		return true
	}
	for _, s := range IterSubmodules(organized, nil) {
		if keep != nil && !keep(s) {
			continue
		}
		if !walk(s.Name, filepath.Join(s.Path, "vineflower")) {
			return
		}
	}
}

// sortedEntries is sorted(os.scandir(dir)) by name; nil when dir cannot be read.
func sortedEntries(dir string) []os.DirEntry {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	entries, _ := f.ReadDir(-1)
	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
	return entries
}
