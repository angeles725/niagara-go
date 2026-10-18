package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// mtime is os.path.getmtime: seconds as a float, like Python's st_mtime.
func mtime(p string) (float64, bool) {
	st, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	t := st.ModTime()
	return float64(t.Unix()) + float64(t.Nanosecond())*1e-9, true
}

// realpath is os.path.realpath: symlinks resolved as far as the path exists.
func realpath(p string) string {
	abs := absPath(p)
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	dir, base := filepath.Split(abs)
	dir = filepath.Clean(dir)
	if dir == abs || base == "" {
		return abs
	}
	return filepath.Join(realpath(dir), base)
}

// listdir is os.listdir: names in directory order (not sorted).
func listdir(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// changedDirs is _changed_dirs: the organized/ root ("organized/") and, per
// module, the first of the module, submodule and vineflower/ directories
// modified after built. Deeper changes are not seen (that would walk every
// source); unreadable entries are skipped.
func changedDirs(organized string, built float64) []string {
	var changed []string
	modules := sortedNames(organized)
	if modules == nil {
		if _, err := listdir(organized); err != nil {
			return changed
		}
	}
	if m, ok := mtime(organized); !ok {
		return changed
	} else if m > built {
		changed = append(changed, "organized/")
	}
	for _, module := range modules {
		mpath := filepath.Join(organized, module)
		if strings.HasPrefix(module, ".") || !pycompat.IsDir(mpath) {
			continue
		}
		type labelled struct{ label, path string }
		dirs := []labelled{{module, mpath}}
		if subs, err := listdir(mpath); err == nil {
			for _, sub := range subs {
				spath := filepath.Join(mpath, sub)
				if pycompat.IsDir(spath) {
					dirs = append(dirs, labelled{module + "/" + sub, spath},
						labelled{module + "/" + sub + "/vineflower", filepath.Join(spath, "vineflower")})
				}
			}
		}
		for _, d := range dirs {
			if m, ok := mtime(d.path); ok && m > built {
				changed = append(changed, d.label)
				break
			}
		}
	}
	return changed
}

// Staleness is corpus.staleness: the warnings when an index no longer
// describes the corpus (built from another corpus path, or older than a
// module, submodule or vineflower/ directory), ending with the rebuild advice.
func Staleness(meta *index.Object, indexPath string) []string {
	return StalenessAt(OrganizedDir(), meta, indexPath)
}

// Staleness is the package-level Staleness against the runtime's corpus
// (OrganizedDir).
func (r *Runtime) Staleness(meta *index.Object, indexPath string) []string {
	return StalenessAt(r.OrganizedDir(), meta, indexPath)
}

// StalenessAt is Staleness against the corpus location organized.
func StalenessAt(organized string, meta *index.Object, indexPath string) []string {
	var warnings []string
	if source := metaSource(meta); source != "" && realpath(source) != realpath(organized) {
		warnings = append(warnings, fmt.Sprintf("index %s was built from %s but the corpus is %s",
			filepath.Base(indexPath), source, organized))
	}
	if built, ok := mtime(indexPath); ok && pycompat.IsDir(organized) {
		if newer := changedDirs(organized, built); len(newer) > 0 {
			warnings = append(warnings, fmt.Sprintf("%d corpus modules had directories added, removed or "+
				"re-created after %s was built (e.g. %s)", len(newer), filepath.Base(indexPath),
				strings.Join(newer[:min(3, len(newer))], ", ")))
		}
	}
	if len(warnings) > 0 {
		warnings = append(warnings, "rebuild with ./reindex.sh")
	}
	return warnings
}

// metaSource is `meta.get("source") or meta.get("source_root")` as a string.
func metaSource(meta *index.Object) string {
	if meta == nil {
		return ""
	}
	for _, k := range []string{"source", "source_root"} {
		if v, ok := meta.Get(k); ok && index.Truthy(v) {
			return PyStr(v)
		}
	}
	return ""
}

// sortedNames is sorted(os.listdir(dir)); nil when dir cannot be read.
func sortedNames(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	names, _ := f.Readdirnames(-1)
	sort.Strings(names)
	return names
}
