package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// The organized/ corpus (module_nav_lib/corpus.py):
//
//	<parent>/module-navigator/        the checkout
//	<parent>/organized/<module>/<submodule>/{vineflower,extracted,<submodule>.jar,...}

// ResolveOrganized returns the corpus root when it exists, else "" and the
// path tried.
func ResolveOrganized() (path, tried string) {
	p := OrganizedDir()
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return p, ""
	}
	return "", p
}

// SourceRoot is corpus.source_root for commands that read sources: the corpus
// root, or "" after an exit-2 error. meta is the _meta block of the index in
// use: when it was built from another corpus that still exists, the hint names it.
func (r *Runtime) SourceRoot(meta *index.Object) string {
	path, tried := ResolveOrganized()
	if path != "" {
		return path
	}
	hint := "Set NAV_CORPUS_BASE to the organized/ directory."
	if rec := metaSource(meta); rec != "" && isDir(rec) && realpath(rec) != realpath(tried) {
		hint = "The index was built from " + rec + ", which exists: export NAV_CORPUS_BASE=" + rec
	}
	r.Error("corpus not found: "+tried, hint)
	return ""
}

// submoduleMarkers: any of these inside organized/<module>/<dir>/ makes <dir> a submodule.
var submoduleMarkers = []string{"vineflower", "decompiled", "extracted", filepath.Join("pipeline", "fase1-recon.json")}

// typeSuffixes are the submodule name suffixes that give its type.
var typeSuffixes = []string{"rt", "wb", "ux", "doc", "se"}

// IsSubmodule is corpus.is_submodule: path holds pipeline output or its own JAR.
func IsSubmodule(path string) bool {
	if isFile(filepath.Join(path, filepath.Base(path)+".jar")) {
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
		if !isDir(mpath) {
			continue
		}
		for _, sub := range sortedNames(mpath) {
			spath := filepath.Join(mpath, sub)
			if !isDir(spath) {
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
	var walk func(sub, dir string)
	walk = func(sub, dir string) {
		var dirs, files []string
		for _, name := range sortedNames(dir) {
			p := filepath.Join(dir, name)
			if isDir(p) {
				dirs = append(dirs, name)
			} else {
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
				out = append(out, JavaFile{sub, p, strings.ReplaceAll(filepath.ToSlash(rel), "\\", "/")})
			}
		}
		for _, name := range dirs {
			p := filepath.Join(dir, name)
			if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
				continue
			}
			walk(sub, p)
		}
	}
	for _, s := range IterSubmodules(organized, nil) {
		walk(s.Name, filepath.Join(s.Path, "vineflower"))
	}
	return out
}

// ClassEntry is one class-index entry with its class name.
type ClassEntry struct {
	Name  string
	Entry *index.Object
}

// TopLevelFiles is corpus.top_level_files: one top-level class-index entry per
// source file, in index order. An entry is inner when it has a truthy `inner`
// or an `outer_class` other than null, "" and "None".
func TopLevelFiles(classes *index.Object) []ClassEntry {
	seen := map[string]bool{}
	var out []ClassEntry
	for i, name := range classes.Keys() {
		entries, _ := classes.Values()[i].([]any)
		for _, ev := range entries {
			e, ok := ev.(*index.Object)
			if !ok {
				continue
			}
			pv, _ := e.Get("path")
			path, _ := pv.(string)
			inner := false
			if v, ok := e.Get("inner"); ok && truthy(v) {
				inner = true
			}
			if v, ok := e.Get("outer_class"); ok {
				if s, isStr := v.(string); v != nil && (!isStr || (s != "" && s != "None")) {
					inner = true
				}
			}
			if inner || !truthy(pv) || seen[path] {
				continue
			}
			seen[path] = true
			out = append(out, ClassEntry{name, e})
		}
	}
	return out
}
