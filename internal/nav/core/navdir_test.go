package core

import (
	"os"
	"path/filepath"
	"testing"
)

// navTree makes <root>/module-navigator (with its inventory index) and returns both.
func navTree(t *testing.T) (root, nav string) {
	t.Helper()
	root = t.TempDir()
	nav = filepath.Join(root, "module-navigator")
	if err := os.MkdirAll(filepath.Join(nav, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nav, NavMarker), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nav, "tools", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, nav
}

func TestFindNavDirAndOrganizedDir(t *testing.T) {
	root, nav := navTree(t)
	elsewhere := t.TempDir()
	t.Cleanup(func() { Executable = os.Executable })
	tests := []struct {
		name, cwd, exe, corpus string
		wantNav, wantOrganized string
	}{
		{"cwd inside the checkout", filepath.Join(nav, "tools", "sub"), filepath.Join(elsewhere, "mnav"), "",
			nav, filepath.Join(root, "organized")},
		{"executable inside the checkout", elsewhere, filepath.Join(nav, "tools", "mnav"), "",
			nav, filepath.Join(root, "organized")},
		{"override wins", filepath.Join(nav, "tools"), filepath.Join(elsewhere, "mnav"), "/corpus/x",
			nav, "/corpus/x"},
		{"nothing found", elsewhere, filepath.Join(elsewhere, "mnav"), "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.cwd)
			t.Setenv("NAV_CORPUS_BASE", tt.corpus)
			Executable = func() (string, error) { return tt.exe, nil }
			if got := FindNavDir(); got != tt.wantNav {
				t.Errorf("FindNavDir() = %q, want %q", got, tt.wantNav)
			}
			if got := OrganizedDir(); got != tt.wantOrganized {
				t.Errorf("OrganizedDir() = %q, want %q", got, tt.wantOrganized)
			}
		})
	}
}
