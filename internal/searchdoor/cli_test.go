package searchdoor

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func TestSplitLicense(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		ok   bool
		rest []string
	}{
		{"route word first", []string{"license", "bacnet"}, true, []string{"bacnet"}},
		{"after a value option", []string{"--limit", "3", "license", "x"}, true, []string{"--limit", "3", "x"}},
		{"after a flag", []string{"--and", "license", "x"}, true, []string{"--and", "x"}},
		{"alone is free text", []string{"license"}, false, []string{"license"}},
		{"after -- is query text", []string{"--", "license", "bacnet"}, false, []string{"--", "license", "bacnet"}},
		{"route then --", []string{"license", "--", "bacnet"}, true, []string{"--", "bacnet"}},
		{"not first positional", []string{"bacnet", "license"}, false, []string{"bacnet", "license"}},
		{"a lone dash is positional", []string{"-", "license"}, false, []string{"-", "license"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, rest := splitLicense(tt.argv)
			if ok != tt.ok || !reflect.DeepEqual(rest, tt.rest) {
				t.Errorf("splitLicense(%q) = %v %q, want %v %q", tt.argv, ok, rest, tt.ok, tt.rest)
			}
		})
	}
}

// envOf is an injected environment: the variables of vars, every other one unset.
func envOf(vars map[string]string) envFunc { return func(name string) string { return vars[name] } }

func TestDefaultPathsHonourEachVariable(t *testing.T) {
	t.Parallel()
	vars := map[string]string{"NSEARCH_ROOT": "/r", "NSEARCH_CORPUS_NAV": "", "NSEARCH_MODULE_NAV": "x/nav.py"}
	// Python joins REPO / "tools" / ... with pathlib: native separators (\r\tools\corpus-nav.py on Windows).
	want := Paths{Blocks: filepath.Join("/r", "tools", "corpus-nav.py"),
		Docs: filepath.Join("/r", "niagara-help", "tools", "niagara_help.py"),
		Code: "x/nav.py", Course: filepath.Join("/r", "sources", "n4-dev-course"), Keys: "/r"}
	if got := defaultPaths(envOf(vars)); got != want {
		t.Errorf("defaultPaths() = %+v, want %+v", got, want)
	}
	vars["NSEARCH_COURSE_KEYS"] = "k"
	if got := defaultPaths(envOf(vars)).Keys; got != "k" {
		t.Errorf("defaultPaths().Keys = %q, want NSEARCH_COURSE_KEYS", got)
	}
}

func TestRootIsFoundAboveTheExecutableThenTheWorkingDirectory(t *testing.T) {
	noEnv := envOf(nil)
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "tools", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "tools", "corpus-nav.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	saved := executable
	t.Cleanup(func() { executable = saved })

	executable = func() (string, error) { return filepath.Join(checkout, "tools", "sub", "nsearch"), nil }
	t.Chdir(elsewhere)
	if got := root(noEnv); got != checkout {
		t.Errorf("root from the executable = %q, want %q", got, checkout)
	}
	executable = func() (string, error) { return filepath.Join(elsewhere, "bin", "nsearch"), nil }
	t.Chdir(filepath.Join(checkout, "tools"))
	if got := root(noEnv); got != checkout {
		t.Errorf("root from the working directory = %q, want %q", got, checkout)
	}
	t.Chdir(elsewhere)
	if got := root(noEnv); got != elsewhere {
		t.Errorf("fallback root = %q, want the executable directory's parent %q", got, elsewhere)
	}
}

// TestMainReadsTheEnvironmentOfItsStdio: the binaries read every path variable
// through cli.Stdio.Env, never the process when the Stdio brings its own.
func TestMainReadsTheEnvironmentOfItsStdio(t *testing.T) {
	t.Setenv("NSEARCH_CORPUS_NAV", filepath.Join(t.TempDir(), "process", "corpus-nav.py"))
	missing := filepath.Join(t.TempDir(), "injected")
	vars := map[string]string{"NSEARCH_ROOT": missing}
	for i, name := range pathEnv {
		vars[name] = filepath.Join(missing, fmt.Sprintf("source%d.py", i))
	}
	var out, errb strings.Builder
	rc := Main([]string{"alarm"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb, Getenv: envOf(vars)})
	if rc != cli.ExitCannotSearch {
		t.Errorf("exit %d, want %d (every source unavailable)\n%s%s", rc, cli.ExitCannotSearch, out.String(), errb.String())
	}
	want := "tool not found at " + pyPath(vars["NSEARCH_CORPUS_NAV"])
	if !strings.Contains(out.String(), want) || strings.Contains(out.String(), "process") {
		t.Errorf("the blocks source does not use the injected path (%q):\n%s", want, out.String())
	}
}
