package slotomatic

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/angeles725/niagara-go/internal/locate"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// WalkFunc receives one directory of a Walk: its path, its subdirectories (the callback may
// reorder or shrink *dirs to prune the walk, like assigning dirs[:] under os.walk) and its files.
type WalkFunc func(dir string, dirs *[]string, files []string)

// Walk is Python's os.walk(root) (top-down, followlinks=False, errors ignored). Entries come in
// directory order, as os.scandir lists them, unless sorted is true (the callers that sort dn and
// files). Paths are joined like os.path.join, so a root such as "./src" keeps its spelling.
func Walk(root string, sorted bool, fn WalkFunc) {
	f, err := os.Open(root)
	if err != nil {
		return
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return
	}
	if sorted {
		sort.Strings(names)
	}
	var dirs, files []string
	for _, n := range names {
		if st, err := os.Stat(Join(root, n)); err == nil && st.IsDir() {
			dirs = append(dirs, n)
		} else {
			files = append(files, n)
		}
	}
	fn(root, &dirs, files)
	for _, d := range dirs {
		p := Join(root, d)
		if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		Walk(p, sorted, fn)
	}
}

// Join is os.path.join(dir, name) for the host (pycompat.Join): no separator is added after one
// that is already there, a rooted name restarts the path, and nothing is cleaned.
func Join(dir, name string) string { return pycompat.Join(dir, name) }

// OrganizedRoot is the organized/ tree of decompiled module sources, or "" when there is none:
// $NIAGARA_ORGANIZED ("", "off" and "0", after strip() and lower(), disable it; otherwise the
// value is used as given, untrimmed like Python's os.path.isdir(env), and a path that is not a
// directory gives none),
// else the organized/ directory of the first checkout (a directory holding .git and organized/)
// found walking up from the working directory, then from the executable's directory. The Python
// tools used <their checkout>/organized; a binary has no checkout (the module-navigator rule of
// decision D24).
func OrganizedRoot() string {
	if env, ok := os.LookupEnv("NIAGARA_ORGANIZED"); ok {
		switch textfold.Lower(pycompat.Strip(env)) {
		case "", "off", "0":
			return ""
		}
		if isDir(env) {
			return env
		}
		return ""
	}
	dir, ok := locate.Find(locate.Options{
		Starts: []locate.Start{locate.FromCwd, locate.FromExecutable},
		Marker: locate.All(locate.Exists(".git"), locate.HasDir("organized")),
	})
	if ok {
		return filepath.Join(dir, "organized")
	}
	return ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
