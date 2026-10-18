// Package help is the Go port of niagara-help (tools/niagara_help.py): the
// Niagara N4 documentation navigator. Main mirrors niagara_help_lib/cli/app.py:
// resolve the repository root and the Niagara version, parse the command
// line with the argparse emulation, run the chosen command and map its
// outcome to the exit code (0 found, 1 searched and matched nothing,
// 2 cannot search).
package help

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/index"
)

// EnvDir names the variable that points at the niagara-help checkout.
const EnvDir = "NIAGARA_HELP_DIR"

// envVersionForTest lets the golden test clear the version variable.
const envVersionForTest = index.EnvVersion

// noVersion lists the commands that run without a resolved version (they
// create or list them), app.py NO_VERSION.
var noVersion = map[string]bool{"versions": true}

// executable is os.Executable; tests replace it to disable the walk-up from
// the test binary.
var executable = os.Executable

// Main is the nh entry point (app.py main).
func Main(args []string, s cli.Stdio) int {
	repo := repoRoot()
	if repo == "" {
		fmt.Fprintln(s.Out, "ERROR: Cannot find niagara-help directory.")
		fmt.Fprintln(s.Out, "Set NIAGARA_HELP_DIR or run from tools/ directory.")
		return 1
	}
	requested, argv, err := popVersionArg(args)
	if err != nil {
		fmt.Fprintln(s.Out, scope.ErrorLine(err))
		return cli.ExitCannotSearch
	}
	if len(argv) > 0 && retired[argv[0]] {
		fmt.Fprintln(s.Err, retiredMessage(argv[0]))
		return cli.ExitCannotSearch
	}
	env := &Env{In: s.In, Out: s.Out, Err: s.Err, RepoRoot: repo}
	if len(argv) == 0 || !noVersion[argv[0]] {
		if code, ok := env.resolveVersion(requested); !ok {
			return code
		}
	}
	p := buildParser()
	ns, code, done := p.Parse(argv, s)
	if done {
		return code
	}
	name := ns.String("command")
	if name == "" {
		p.PrintHelp(s.Out)
		return cli.ExitFound
	}
	return env.run(registry[name], ns)
}

// repoRoot finds the niagara-help checkout: $NIAGARA_HELP_DIR, else the first
// folder holding indexes/ walking up from the executable, else walking up
// from the working directory. (Python only knows the parent of tools/; a Go
// binary has no tools/ folder, so this is an intended difference.)
func repoRoot() string {
	if d := os.Getenv(EnvDir); d != "" {
		return d
	}
	if exe, err := executable(); err == nil && exe != "" {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		if r := findUp(filepath.Dir(exe)); r != "" {
			return r
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return findUp(wd)
	}
	return ""
}

// findUp returns the first of dir and its ancestors that holds an indexes/ folder.
func findUp(dir string) string {
	for {
		if st, err := os.Stat(filepath.Join(dir, "indexes")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

const versionEq = "--niagara-version="

// popVersionArg removes every `--niagara-version X` / `--niagara-version=X`
// from argv, wherever it appears, and returns the last value (versions.py
// pop_version_arg). The flag is resolved before argparse sees the line.
func popVersionArg(argv []string) (value string, rest []string, err error) {
	rest = []string{}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "--niagara-version":
			if i+1 >= len(argv) {
				return "", nil, &index.UnknownVersionError{Msg: "--niagara-version needs a value"}
			}
			value = argv[i+1]
			i++
		case strings.HasPrefix(arg, versionEq):
			value = arg[len(versionEq):]
		default:
			rest = append(rest, arg)
		}
	}
	return value, rest, nil
}

// resolveVersion maps the requested version to its data root (app.py
// resolve_base_dir). On failure it prints the error and returns the exit code.
func (e *Env) resolveVersion(requested string) (int, bool) {
	id, root, err := scope.Resolve(e.RepoRoot, requested, os.Getenv)
	if err != nil {
		// An unknown version is exit 2 in Python; a malformed registry is a
		// traceback there and the same ERROR line here (intended difference).
		for _, l := range scope.VersionLines(err) {
			fmt.Fprintln(e.Out, l)
		}
		return cli.ExitCannotSearch, false
	}
	e.VersionID, e.BaseDir = id, root
	return 0, true
}

// run executes a parsed command and maps its outcome to the exit code: an
// explicit exit status, "ERROR: ..." on stderr with exit 2 for any other
// error, else 1 when a lookup recorded a miss and 0 otherwise.
func (e *Env) run(c *command, ns *argparse.Namespace) int {
	if c.run == nil {
		fmt.Fprintln(e.Err, notPortedError(c.name).Error())
		return cli.ExitCannotSearch
	}
	err := c.run(e, ns)
	var code ExitCode
	var np notPortedError
	switch {
	case errors.As(err, &code):
		return int(code)
	case errors.As(err, &np):
		fmt.Fprintln(e.Err, np.Error())
		return cli.ExitCannotSearch
	case err != nil:
		fmt.Fprintln(e.Err, scope.ErrorLine(err))
		return cli.ExitCannotSearch
	case e.misses > 0:
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}
