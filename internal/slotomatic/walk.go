package slotomatic

import (
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/locate"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

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
		if pycompat.IsDir(env) {
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
