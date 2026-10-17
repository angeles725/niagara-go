package nav

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func TestEveryOrderedCommandIsRegistered(t *testing.T) {
	if len(Order) != 134 {
		t.Fatalf("Order has %d commands, want 134", len(Order))
	}
	if got := registry.Names(); len(got) != len(Order) {
		t.Fatalf("%d commands registered, want %d", len(got), len(Order))
	}
	if _, err := BuildParser(); err != nil {
		t.Fatal(err)
	}
}

func TestUnportedCommandExitsTwo(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	var out, errb bytes.Buffer
	code := Main([]string{"--base-dir", t.TempDir(), "class", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != "mnav: 'class' is not ported yet\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestNoBaseDirExitsOne(t *testing.T) {
	t.Chdir(t.TempDir())
	executable = func() (string, error) { return filepath.Join(t.TempDir(), "mnav"), nil }
	t.Cleanup(func() { executable = os.Executable })
	var out, errb bytes.Buffer
	code := Main([]string{"class", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	want := "ERROR: Cannot detect base directory.\nUse --base-dir or run from module-navigator/tools/\n"
	if code != cli.ExitNoMatch || out.String() != want || errb.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

// navTree makes <root>/module-navigator (with its inventory index) and returns both.
func navTree(t *testing.T) (root, nav string) {
	t.Helper()
	root = t.TempDir()
	nav = filepath.Join(root, "module-navigator")
	if err := os.MkdirAll(filepath.Join(nav, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nav, navMarker), []byte("{}"), 0o644); err != nil {
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
	t.Cleanup(func() { executable = os.Executable })
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
			executable = func() (string, error) { return tt.exe, nil }
			if got := FindNavDir(); got != tt.wantNav {
				t.Errorf("FindNavDir() = %q, want %q", got, tt.wantNav)
			}
			if got := OrganizedDir(); got != tt.wantOrganized {
				t.Errorf("OrganizedDir() = %q, want %q", got, tt.wantOrganized)
			}
		})
	}
}

func TestEpilogListsEveryCommandOnce(t *testing.T) {
	epilog, err := groupedCommands()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, line := range strings.Split(epilog, "\n") {
		if strings.HasPrefix(line, "    ") {
			seen[strings.Fields(line)[0]]++
		}
	}
	for _, name := range Order {
		if seen[name] != 1 {
			t.Errorf("%s listed %d times", name, seen[name])
		}
	}
}
