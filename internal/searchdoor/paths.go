package searchdoor

import (
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/locate"
)

// envFunc reads one environment variable: cli.Stdio.Env in the binaries, a
// map in the tests that inject their environment.
type envFunc = func(string) string

// pathEnv names the variable that re-points each source (the variables the
// Python front door read).
var pathEnv = [5]string{"NSEARCH_CORPUS_NAV", "NSEARCH_NIAGARA_HELP", "NSEARCH_MODULE_NAV", "NSEARCH_COURSE_ROOT",
	"NSEARCH_COURSE_KEYS"}

// executable is os.Executable with symlinks resolved; tests replace it.
var executable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// root locates the niagara-research checkout (see the package doc).
func root(getenv envFunc) string {
	if r, ok := locate.Find(locate.Options{Env: []string{"NSEARCH_ROOT"}, Getenv: getenv,
		Starts: []locate.Start{locate.FromExecutable, locate.FromCwd}, Marker: locate.ResearchCheckout,
		Executable: executable}); ok {
		return r
	}
	exeDir := ""
	if exe, err := executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	return filepath.Dir(exeDir)
}

// defaultPaths are the backends of the checkout (each one's checkout directory,
// no Python shim), each re-pointed by its NSEARCH_* variable when that is set
// and not empty. The checkout is only located when a path still needs it.
func defaultPaths(getenv envFunc) Paths {
	vals := [len(pathEnv)]string{}
	need := false
	for i, name := range pathEnv {
		vals[i] = getenv(name)
		need = need || vals[i] == ""
	}
	if need {
		r := root(getenv)
		defaults := [len(pathEnv)]string{r, filepath.Join(r, "niagara-help"),
			filepath.Join(r, "module-navigator"), filepath.Join(r, "sources", "n4-dev-course"), r}
		for i := range vals {
			if vals[i] == "" {
				vals[i] = defaults[i]
			}
		}
	}
	return Paths{Blocks: vals[0], Docs: vals[1], Code: vals[2], Course: vals[3], Keys: vals[4]}
}
