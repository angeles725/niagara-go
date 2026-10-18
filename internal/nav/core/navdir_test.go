package core

import (
	"os"
	"path/filepath"
	"strings"
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
			t.Setenv("MODULE_NAVIGATOR_DIR", "")
			t.Setenv("NAV_DIR", "")
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

// TestLocateNavDirOverride: MODULE_NAVIGATOR_DIR (or its alias NAV_DIR)
// naming a directory with indexes/ wins over the search; naming anything
// else is an error that names the variable, never a fallback to the search;
// unset leaves the search unchanged.
func TestLocateNavDirOverride(t *testing.T) {
	_, nav := navTree(t)
	other := t.TempDir() // a checkout found only through the variable
	if err := os.MkdirAll(filepath.Join(other, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Executable = os.Executable })
	Executable = func() (string, error) { return filepath.Join(t.TempDir(), "mnav"), nil }
	t.Chdir(filepath.Join(nav, "tools")) // the search alone finds nav
	tests := []struct {
		name, modDir, navDir string
		want, wantErr        string
	}{
		{"unset: the search", "", "", nav, ""},
		{"MODULE_NAVIGATOR_DIR set and valid", other, "", other, ""},
		{"NAV_DIR alias set and valid", "", other, other, ""},
		{"a relative value is made absolute", "..", "", nav, ""},
		{"MODULE_NAVIGATOR_DIR wins over NAV_DIR", other, filepath.Join(other, "missing"), other, ""},
		{"set to a missing directory", filepath.Join(other, "missing"), "", "", "MODULE_NAVIGATOR_DIR"},
		{"set to a directory without indexes/", t.TempDir(), "", "", "MODULE_NAVIGATOR_DIR"},
		{"alias set to a missing directory", "", filepath.Join(other, "missing"), "", "NAV_DIR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MODULE_NAVIGATOR_DIR", tt.modDir)
			t.Setenv("NAV_DIR", tt.navDir)
			got, err := LocateNavDir()
			if got != tt.want {
				t.Errorf("LocateNavDir() = %q, want %q", got, tt.want)
			}
			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.HasPrefix(err.Error(), tt.wantErr+"=")) {
				t.Errorf("error %v, want one naming %s", err, tt.wantErr)
			}
			if fd := FindNavDir(); fd != tt.want {
				t.Errorf("FindNavDir() = %q, want %q (never the search after a bad variable)", fd, tt.want)
			}
		})
	}
}
