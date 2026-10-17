package searchdoor

import (
	"os"
	"path/filepath"
)

// pathEnv names the variable that re-points each source (the Python seam).
var pathEnv = [4]string{"NSEARCH_CORPUS_NAV", "NSEARCH_NIAGARA_HELP", "NSEARCH_MODULE_NAV", "NSEARCH_COURSE_ROOT"}

// marker identifies a niagara-research checkout.
var marker = filepath.Join("tools", "corpus-nav.py")

// executable is os.Executable with symlinks resolved; tests replace it.
var executable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// findUp returns the first directory at or above dir holding the marker.
func findUp(dir string) (string, bool) {
	for {
		if st, err := os.Stat(filepath.Join(dir, marker)); err == nil && !st.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// root locates the niagara-research checkout (see the package doc).
func root() string {
	if r := os.Getenv("NSEARCH_ROOT"); r != "" {
		return r
	}
	exeDir := ""
	if exe, err := executable(); err == nil {
		exeDir = filepath.Dir(exe)
		if r, ok := findUp(exeDir); ok {
			return r
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if r, ok := findUp(wd); ok {
			return r
		}
	}
	return filepath.Dir(exeDir)
}

// defaultPaths are the backends of the checkout, each re-pointed by its
// NSEARCH_* variable when that is set and not empty. The checkout is only
// located when a path still needs it.
func defaultPaths() Paths {
	vals := [4]string{}
	need := false
	for i, name := range pathEnv {
		vals[i] = os.Getenv(name)
		need = need || vals[i] == ""
	}
	if need {
		r := root()
		defaults := [4]string{filepath.Join(r, "tools", "corpus-nav.py"),
			filepath.Join(r, "niagara-help", "tools", "niagara_help.py"),
			filepath.Join(r, "module-navigator", "tools", "module_nav.py"),
			filepath.Join(r, "sources", "n4-dev-course")}
		for i := range vals {
			if vals[i] == "" {
				vals[i] = defaults[i]
			}
		}
	}
	return Paths{Blocks: vals[0], Docs: vals[1], Code: vals[2], Course: vals[3]}
}
