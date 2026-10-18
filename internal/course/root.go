package course

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/course/key"
	"github.com/angeles725/niagara-go/internal/locate"
)

// RootEnv names the variable that points at a niagara-research checkout.
const RootEnv = key.RootEnv

// executable is os.Executable; tests stub it so the walk from the binary is hermetic.
var executable = os.Executable

// findRoot applies --root, then NIAGARA_RESEARCH_DIR, then the walks from the working
// directory and from the executable. A named directory without labs.md is an error, not a
// reason to keep looking.
func findRoot(flag string, hasFlag bool) (string, error) {
	if hasFlag {
		if !key.HasMarker(flag) {
			return "", fmt.Errorf("--root %s has no %s", flag, key.LabsFile)
		}
		return flag, nil
	}
	if env := strings.TrimSpace(os.Getenv(RootEnv)); env != "" {
		if !key.HasMarker(env) {
			return "", fmt.Errorf("%s=%s has no %s", RootEnv, env, key.LabsFile)
		}
		return env, nil
	}
	if dir, ok := locate.Find(locate.Options{
		Starts:     []locate.Start{locate.FromCwd, locate.FromExecutable},
		Marker:     key.HasMarker,
		Executable: executable,
	}); ok {
		return dir, nil
	}
	return "", fmt.Errorf("cannot find %s: pass --root DIR, set %s, or run inside a niagara-research checkout",
		key.LabsFile, RootEnv)
}
