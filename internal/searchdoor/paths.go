package searchdoor

import (
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/locate"
)

// envFunc reads one environment variable: cli.Stdio.Env in the binaries, a
// map in the tests that inject their environment.
type envFunc = func(string) string

// pathEnv names the variable that re-points each source (the Python seam).
var pathEnv = [5]string{"NSEARCH_CORPUS_NAV", "NSEARCH_NIAGARA_HELP", "NSEARCH_MODULE_NAV", "NSEARCH_COURSE_ROOT",
	"NSEARCH_COURSE_KEYS"}

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

// isCheckout holds the marker as any non-directory (the Python seam's is_file
// would also refuse a device; no checkout holds one).
func isCheckout(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, marker))
	return err == nil && !st.IsDir()
}

// root locates the niagara-research checkout (see the package doc).
func root(getenv envFunc) string {
	if r, ok := locate.Find(locate.Options{Env: []string{"NSEARCH_ROOT"}, Getenv: getenv,
		Starts: []locate.Start{locate.FromExecutable, locate.FromCwd}, Marker: isCheckout,
		Executable: executable}); ok {
		return r
	}
	exeDir := ""
	if exe, err := executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	return filepath.Dir(exeDir)
}

// defaultPaths are the backends of the checkout, each re-pointed by its
// NSEARCH_* variable when that is set and not empty. The checkout is only
// located when a path still needs it.
func defaultPaths(getenv envFunc) Paths {
	vals := [len(pathEnv)]string{}
	need := false
	for i, name := range pathEnv {
		vals[i] = getenv(name)
		need = need || vals[i] == ""
	}
	if need {
		r := root(getenv)
		defaults := [len(pathEnv)]string{filepath.Join(r, "tools", "corpus-nav.py"),
			filepath.Join(r, "niagara-help", "tools", "niagara_help.py"),
			filepath.Join(r, "module-navigator", "tools", "module_nav.py"),
			filepath.Join(r, "sources", "n4-dev-course"), r}
		for i := range vals {
			if vals[i] == "" {
				vals[i] = defaults[i]
			}
		}
	}
	return Paths{Blocks: vals[0], Docs: vals[1], Code: vals[2], Course: vals[3], Keys: vals[4]}
}
