package help

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/pyref"
)

// maintParityVar selects the real versions of TestParityMaintReal (a
// path-list of registered version ids of the niagara-help checkout). The
// test runs only on request: minutes of jar reading over /mnt/c.
const maintParityVar = "NH_PARITY_MAINT"

// maintParityCases are the command lines compared, after
// "--niagara-version <id>".
var maintParityCases = [][]string{
	{"coverage", "--help"},
	{"coverage"},
	{"coverage", "alarm-rt"},
	{"coverage", "nosuchmodule"},
	{"coverage", "--doc-jars"},
	{"coverage", "--sweep"},
	{"coverage", "--sweep", "--doc-jars"},
}

// maintOutcome is what one side printed, its exit status and the
// jar-listing cache it left (relative path -> bytes).
type maintOutcome struct {
	stdout, stderr string
	code           int
	cache          map[string]string
}

// TestParityMaintReal runs each maintParityCases line through the pinned
// Python niagara_help.py and through nh's Main, on one temporary checkout
// whose registry holds the version with its absolute data root and real
// install, and compares stdout, stderr, the exit status and the jar-listing
// cache each side writes (both start from niagara-help's cache of the
// install, when it has one). Nothing is written into niagara-help: a stale
// indexes/fulltext.sqlite is replaced by an index built in a temporary
// XDG_CACHE_HOME, open_index's second place, which both sides read.
func TestParityMaintReal(t *testing.T) {
	list := os.Getenv(maintParityVar)
	if list == "" {
		t.Skipf("real-data parity runs only on request: set %s=<version id>[:...]", maintParityVar)
	}
	tools := filepath.Join(pyref.Dir(t, "niagara-help"), "tools")
	python := pyref.Python(t)
	help := os.Getenv(EnvDir)
	if help == "" {
		home, _ := os.UserHomeDir()
		help = filepath.Join(home, "niagara-research", "niagara-help")
	}
	for _, vid := range filepath.SplitList(list) {
		if vid == "" {
			continue
		}
		t.Run(vid, func(t *testing.T) {
			repo, base, install := maintParityRepo(t, help, vid)
			freshFulltextElsewhere(t, base)
			seed, _ := helpbuild.CachePath(help, install)
			for _, args := range maintParityCases {
				argv := append([]string{"--niagara-version", vid}, args...)
				what := strings.Join(args, " ")
				start := time.Now()
				py := maintRun(t, repo, install, seed, func() (string, string, int) {
					return runPython(t, python, tools, repo, argv)
				})
				pyTook := time.Since(start)
				start = time.Now()
				gol := maintRun(t, repo, install, seed, func() (string, string, int) {
					t.Setenv(EnvDir, repo)
					t.Setenv(envVersionForTest, "")
					t.Setenv("COLUMNS", "100")
					var out, errb bytes.Buffer
					code := Main(argv, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
					return out.String(), errb.String(), code
				})
				goTook := time.Since(start)
				compareMaint(t, what, py, gol)
				t.Logf("%s: exit %d, %d stdout bytes; python %.1f s, go %.1f s", what, py.code, len(py.stdout),
					pyTook.Seconds(), goTook.Seconds())
			}
		})
	}
}

// maintParityRepo writes a temporary checkout whose registry has vid alone,
// as the default, with its data root made absolute; it returns the
// checkout, the data root and the install.
func maintParityRepo(t *testing.T, help, vid string) (repo, base, install string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(help, "versions", "registry.json"))
	if err != nil {
		t.Fatalf("niagara-help registry: %v", err)
	}
	var reg struct {
		Versions map[string]map[string]any `json:"versions"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Versions[vid]
	if !ok {
		t.Fatalf("version %s is not registered in %s", vid, help)
	}
	root, _ := entry["data_root"].(string)
	install, _ = entry["install_dir"].(string)
	if cd, ok := entry["course_dir"].(string); ok && !filepath.IsAbs(cd) {
		t.Fatalf("%s binds the course %s relative to its checkout: not supported here", vid, cd)
	}
	base = filepath.Clean(filepath.Join(help, filepath.FromSlash(root)))
	entry["data_root"] = base
	if st, err := os.Stat(install); err != nil || !st.IsDir() {
		t.Fatalf("install of %s not mounted: %s", vid, install)
	}
	out, err := json.MarshalIndent(map[string]any{"default": vid, "versions": map[string]any{vid: entry}}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	repo = t.TempDir()
	write(t, filepath.Join(repo, "versions", "registry.json"), string(out))
	return repo, base, install
}

// freshFulltextElsewhere makes sure both sides find an up-to-date full-text
// index of base without writing into base.
func freshFulltextElsewhere(t *testing.T, base string) {
	t.Helper()
	if _, err := helpbuild.FreshFulltextPath(base); err == nil {
		return
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	start := time.Now()
	res, err := ftbuild.Build(base, ftbuild.Options{Output: fts.CachePath(base)})
	if err != nil {
		t.Fatalf("building the full-text index of %s in a temporary cache: %v", base, err)
	}
	t.Logf("%s: indexes/fulltext.sqlite is stale; built %d documents in a temporary XDG_CACHE_HOME (%.1f s)",
		base, res.Documents, time.Since(start).Seconds())
	if _, err := helpbuild.FreshFulltextPath(base); err != nil {
		t.Fatal(err)
	}
}

// maintRun resets repo's jar-listing cache to seed (none when seed does
// not exist), runs one side and returns its outcome with the cache it left.
func maintRun(t *testing.T, repo, install, seed string, side func() (string, string, int)) maintOutcome {
	t.Helper()
	cacheDir := filepath.Join(repo, ".cache")
	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(seed); err == nil {
		dst, err := helpbuild.CachePath(repo, install)
		if err != nil {
			t.Fatal(err)
		}
		write(t, dst, string(body))
	}
	stdout, stderr, code := side()
	cache := map[string]string{}
	filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			body, rerr := os.ReadFile(path)
			if rerr != nil {
				t.Fatal(rerr)
			}
			rel, _ := filepath.Rel(cacheDir, path)
			cache[rel] = string(body)
		}
		return nil
	})
	return maintOutcome{stdout, stderr, code, cache}
}

// runPython runs the pinned niagara_help.py (its Python, not the Go shim:
// NIAGARA_HELP_PYTHON=1, and no Go binary to exec) on repo.
func runPython(t *testing.T, python, tools, repo string, argv []string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(python, append([]string{"-B", filepath.Join(tools, "niagara_help.py")}, argv...)...)
	cmd.Dir = repo
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "NIAGARA_HELP_") && !strings.HasPrefix(kv, "COLUMNS=") &&
			!strings.HasPrefix(kv, "NIAGARA_GO_BIN_DIR=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(append(env, pyref.Env("niagara-help")...), EnvDir+"="+repo, "COLUMNS=100",
		"NIAGARA_GO_BIN_DIR=/nonexistent", "PYTHONHASHSEED=0")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("python %v: %v", argv, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errb.String(), code
}

func compareMaint(t *testing.T, what string, py, gol maintOutcome) {
	t.Helper()
	if py.code != gol.code {
		t.Errorf("%s: exit python %d, go %d", what, py.code, gol.code)
	}
	for _, s := range []struct{ name, py, gol string }{{"stdout", py.stdout, gol.stdout}, {"stderr", py.stderr, gol.stderr}} {
		if s.py != s.gol {
			t.Errorf("%s: %s differs: %s", what, s.name, firstDiff(s.py, s.gol))
		}
	}
	if len(py.cache) != len(gol.cache) {
		t.Errorf("%s: jar-listing cache: python %d files, go %d", what, len(py.cache), len(gol.cache))
	}
	for rel, body := range py.cache {
		if gol.cache[rel] != body {
			t.Errorf("%s: jar-listing cache %s differs", what, rel)
		}
	}
}

// firstDiff names the first differing line of two outputs.
func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < max(len(al), len(bl)); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y || i >= len(al) || i >= len(bl) {
			return fmt.Sprintf("line %d: python %q, go %q (%d / %d lines)", i+1, x, y, len(al), len(bl))
		}
	}
	return "same lines"
}
