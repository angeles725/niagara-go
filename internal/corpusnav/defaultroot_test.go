package corpusnav

import (
	"os"
	"path/filepath"
	"testing"
)

// With CORPUS_NAV_ROOT unset and an executable outside any checkout (the test
// binary), the corpus is found by walking up from the working directory with the
// shared checkout marker (locate.ResearchCheckout), so a direct call of the
// installed binary inside the checkout searches it.
func TestDefaultRootFindsTheCheckoutFromTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"CATALOG.md", "niagara-mental-model-bloque1.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(root, "notes", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	got, err := defaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	if g, _ := filepath.EvalSymlinks(got); g != want {
		t.Errorf("defaultRoot() = %q, want %q", got, root)
	}
}

// Outside any checkout the executable's grandparent stays the answer, so the
// cannot-search message still names a concrete root.
func TestDefaultRootOutsideACheckoutIsTheExecutableGrandparent(t *testing.T) {
	t.Chdir(t.TempDir())
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	got, err := defaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Dir(filepath.Dir(exe)); got != want {
		t.Errorf("defaultRoot() = %q, want %q", got, want)
	}
}
