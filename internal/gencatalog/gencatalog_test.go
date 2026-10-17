package gencatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are niagara-research tools/tests/fixtures/golden-cli/gen-catalog/*.json (the
// Python output); the repository root is the case's working directory (the fixture corpus).

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
