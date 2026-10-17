package course

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RootEnv names the variable that points at a niagara-research checkout.
const RootEnv = "NIAGARA_RESEARCH_DIR"

// executable is os.Executable; tests stub it so the walk from the binary is hermetic.
var executable = os.Executable

func hasMarker(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(labsFile)))
	return err == nil && st.Mode().IsRegular()
}

// walkUp returns the first directory at or above dir that holds labs.md, or "".
func walkUp(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if hasMarker(dir) {
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
		if !hasMarker(flag) {
			return "", fmt.Errorf("--root %s has no %s", flag, labsFile)
		}
		return flag, nil
	}
	if env := strings.TrimSpace(os.Getenv(RootEnv)); env != "" {
		if !hasMarker(env) {
			return "", fmt.Errorf("%s=%s has no %s", RootEnv, env, labsFile)
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
		labsFile, RootEnv)
}

// load parses the three answer keys under root. Missing files and parse errors are returned
// as lines to print; a corpus is returned only when both lists are empty.
func load(root string) (*Corpus, []string) {
	var problems []string
	data := map[string][]string{}
	for _, f := range sourceFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		switch {
		case os.IsNotExist(err):
			problems = append(problems, "missing source "+f)
		case err != nil:
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", f, err))
		default:
			data[f] = splitLines(raw)
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	c := newCorpus()
	parseLabs(c, data[labsFile])
	parseReview(c, data[reviewFile])
	parseExam(c, data[examFile])
	for _, e := range c.sortedErrors() {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", e.file, e.line, e.msg))
	}
	if len(problems) > 0 {
		return nil, append(problems, fmt.Sprintf("%s; nothing searched", plural(len(c.errs), "parse error")))
	}
	return c, nil
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
