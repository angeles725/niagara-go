package course

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// RootEnv names the variable that points at a niagara-research checkout.
const RootEnv = key.RootEnv

// executable is os.Executable; tests stub it so the walk from the binary is hermetic.
var executable = os.Executable

// walkUp returns the first directory at or above dir that holds labs.md, or "".
func walkUp(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if key.HasMarker(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

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
	if cwd, err := os.Getwd(); err == nil {
		if dir := walkUp(cwd); dir != "" {
			return dir, nil
		}
	}
	if exe, err := executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		if dir := walkUp(filepath.Dir(exe)); dir != "" {
			return dir, nil
		}
	}
	return "", fmt.Errorf("cannot find %s: pass --root DIR, set %s, or run inside a niagara-research checkout",
		key.LabsFile, RootEnv)
}
