package gencatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are niagara-research tools/tests/fixtures/golden-cli/gen-catalog/*.json (the
// Python output, converted by the retired testdata/convert_golden.py); the repository root is the case's
// working directory (the fixture corpus).

func useCwdAsRoot(t *testing.T) {
	t.Helper()
	saved := repoRoot
	repoRoot = os.Getwd
	t.Cleanup(func() { repoRoot = saved })
}

func TestGolden(t *testing.T) {
	useCwdAsRoot(t)
	golden.Run(t, "testdata", Main)
}

// TestGoldenCatalogStates runs the written, fresh and stale cases (testdata/states/<state>)
// on a temporary copy of the corpus whose CATALOG.md is absent, the Python one, or the Python
// one with block 9's row renamed to gone.md (the states of the Python suite).
func TestGoldenCatalogStates(t *testing.T) {
	useCwdAsRoot(t)
	catalog := expected(t, filepath.Join("testdata", "expected"), "CATALOG.md")
	for state, content := range map[string]string{
		"written": "",
		"fresh":   catalog,
		"stale":   strings.Replace(catalog, "| 9 | [niagara-mental-model-bloque9.md]", "| 8 | [gone.md]", 1),
	} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.CopyFS(filepath.Join(dir, "corpus"), os.DirFS(filepath.Join("testdata", "corpus"))); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(filepath.Join(dir, "golden"), os.DirFS(filepath.Join("testdata", "states", state, "golden"))); err != nil {
				t.Fatal(err)
			}
			if content != "" {
				if err := os.WriteFile(filepath.Join(dir, "corpus", catalogName), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden.Run(t, dir, Main)
		})
	}
}

func run(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if errb.Len() > 0 {
		t.Errorf("%v: stderr %q", args, errb.String())
	}
	return out.String(), code
}

func expected(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestWriteCheckStale walks the CATALOG.md states of the Python golden: absent, written
// (the zero-argument run writes exactly what --stdout prints), fresh, stale.
func TestWriteCheckStale(t *testing.T) {
	useCwdAsRoot(t)
	dir, err := filepath.Abs(filepath.Join("testdata", "expected"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "corpus"))); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	out, code := run(t)
	if code != 0 || out != expected(t, dir, "written.stdout") {
		t.Fatalf("write: exit %d, stdout %q", code, out)
	}
	written, err := os.ReadFile(filepath.Join(root, catalogName))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != expected(t, dir, "CATALOG.md") {
		t.Errorf("CATALOG.md differs from the Python one")
	}
	if printed, _ := run(t, "--stdout"); printed != string(written) {
		t.Errorf("--stdout differs from the written catalog")
	}
	if out, code := run(t, "--check"); code != 0 || out != expected(t, dir, "fresh-check.stdout") {
		t.Errorf("fresh --check: exit %d, stdout %q", code, out)
	}
	stale := strings.Replace(string(written), "| 9 | [niagara-mental-model-bloque9.md]", "| 8 | [gone.md]", 1)
	if err := os.WriteFile(filepath.Join(root, catalogName), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, "--check"); code != 1 || out != expected(t, dir, "stale-check.stdout") {
		t.Errorf("stale --check: exit %d, stdout %q", code, out)
	}
}

func TestNormalizeTitle(t *testing.T) {
	for h1, want := range map[string]string{
		"# Niagara N4 — Mental Model · Bloque 27":      "",
		"# Niagara N4 — Bloque 4: Alarm routing":       "Alarm routing",
		"# Bloque #5 — Histories and trends":           "Histories and trends",
		"# bloque ti - lower case":                     "lower case",
		"# Bloque 10 —":                                "",
		"# Plain English title without a prefix":       "Plain English title without a prefix",
		"# Niagara N4 — Mental Model · Bloques 1-3: x": "Niagara N4 — Mental Model · Bloques 1-3: x",
	} {
		if got := normalizeTitle(h1); got != want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", h1, got, want)
		}
	}
}

// TestUnreadableIsCannotCheck: Python dies with a traceback (exit 1, the same code as a
// stale catalog under --check). Intended difference: an unreadable block or CATALOG.md
// is one stderr line in the OSError.strerror form and exit 2, "could not check".
func TestUnreadableIsCannotCheck(t *testing.T) {
	useCwdAsRoot(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "corpus"))); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	bad := filepath.Join(root, "niagara-mental-model-bloque77.md")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	reason := "Is a directory"
	if runtime.GOOS == "windows" {
		reason = "Permission denied" // the C runtime's open() refuses a directory with EACCES
	}
	for _, args := range [][]string{{"--check"}, {"--stdout"}, {}} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		if want := "gen-catalog: cannot read " + bad + ": " + reason + "\n"; code != 2 || out.Len() != 0 || errb.String() != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errb.String())
		}
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(root, catalogName)
	if err := os.WriteFile(catalog, []byte("\xff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := Main([]string{"--check"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if want := "gen-catalog: cannot read " + catalog + ": not valid UTF-8\n"; code != 2 || out.Len() != 0 || errb.String() != want {
		t.Errorf("catalog: exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

// TestRefusesOutsideACheckout: a root that holds no main-corpus block (the binary run
// from its install folder, whose grandparent is not niagara-research) is not a catalog
// of zero blocks. Every mode refuses with exit 3 (environment: no checkout), names the
// root on stderr, prints nothing on stdout and writes no CATALOG.md.
func TestRefusesOutsideACheckout(t *testing.T) {
	root := t.TempDir()
	saved := repoRoot
	repoRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { repoRoot = saved })
	for _, args := range [][]string{{}, {"--check"}, {"--stdout"}} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		want := "gen-catalog: no niagara-mental-model-bloque*.md block file under " + root +
			" -- not a niagara-research checkout; nothing written\n"
		if code != 3 || out.Len() != 0 || errb.String() != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errb.String())
		}
		if _, err := os.Stat(filepath.Join(root, catalogName)); !os.IsNotExist(err) {
			t.Fatalf("%v: %s written outside a checkout (stat: %v)", args, catalogName, err)
		}
	}
}

// TestUnlistableRootIsEnvironment: a repository root this user cannot list is not a
// checkout of zero blocks. Every mode exits 3 (environment) naming the root and the OS
// reason, prints nothing on stdout and writes nothing.
func TestUnlistableRootIsEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	root := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	if _, err := os.ReadDir(root); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	saved := repoRoot
	repoRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { repoRoot = saved })
	for _, args := range [][]string{{}, {"--check"}, {"--stdout"}} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		want := "gen-catalog: cannot list " + root + ": Permission denied\n"
		if code != 3 || out.Len() != 0 || errb.String() != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errb.String())
		}
	}
}
