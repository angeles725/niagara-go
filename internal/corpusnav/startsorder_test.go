package corpusnav

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeCheckout makes a directory that locate.ResearchCheckout accepts.
func fakeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"CATALOG.md", "niagara-mental-model-bloque1.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func resolved(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// defaultRoot searches from the executable before the working directory: an
// installed binary inside one checkout serves that checkout even when it runs
// from another one; outside any checkout, the working directory's checkout wins.
func TestDefaultRootPrefersTheExecutablesCheckoutOverTheCwds(t *testing.T) {
	exeCheckout, cwdCheckout := fakeCheckout(t), fakeCheckout(t)
	if err := os.MkdirAll(filepath.Join(exeCheckout, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	tests := []struct {
		name string
		exe  string
		want string
	}{
		{"executable in a checkout wins", filepath.Join(exeCheckout, "bin", "cnav"), exeCheckout},
		{"executable outside a checkout falls back to cwd", filepath.Join(outside, "bin", "cnav"), cwdCheckout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe := tt.exe
			old := executable
			t.Cleanup(func() { executable = old })
			executable = func() (string, error) { return exe, nil }
			t.Chdir(cwdCheckout)
			got, err := defaultRoot()
			if err != nil {
				t.Fatal(err)
			}
			if g, w := resolved(t, got), resolved(t, tt.want); g != w {
				t.Errorf("defaultRoot() = %q, want %q", g, w)
			}
		})
	}
}
