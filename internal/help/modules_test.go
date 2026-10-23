package help

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot is a data root outside any registry whose only index is the
// module-index.json fixture (testdata/indexes/).
func moduleRoot(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "module-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "indexes", "module-index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// runModule runs `module` on the fixture root: stdout with the root shown as
// <DIR>, and the misses counted.
func runModule(t *testing.T, p ModuleParams) (string, int) {
	t.Helper()
	root := moduleRoot(t)
	var out bytes.Buffer
	e := &Env{BaseDir: root, Out: &out}
	if err := e.Module(p); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(out.String(), root, "<DIR>"), e.misses
}

func TestModuleSummary(t *testing.T) {
	got, misses := runModule(t, ModuleParams{Name: "kitControl-rt"})
	want := `MODULE: kitControl-rt
JAR: kitControl-rt.jar
PART: kitControl (runtime profile rt)
VENDOR: Tridium 4.15.6.20
DESCRIPTION: Library of Control Components

DEPENDENCIES (2):
  baja           Tridium 4.15.6
  control-rt     Tridium 4.15.6

REVERSE DEPENDENCIES (2):
  kitControl-ux  requires 4.15.6
  kitControl-wb  requires 4.15.6

TYPES: 2   (nh module kitControl-rt --types)
AGENTS: 0  (nh module kitControl-rt --agents)
DEFS: 0    (nh module kitControl-rt --defs)
ADAPTERS: 0

JAVA PERMISSIONS (2):
  station  java.lang.RuntimePermission "setIO"
  all      java.util.PropertyPermission "*" read
`
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestModuleResolution: an exact name, then the name in any letter case, then
// the module part name (every runtime profile), then a glob; more than one
// match is a listing, not a guess.
func TestModuleResolution(t *testing.T) {
	listing := `3 modules matching 'kitControl':

  kitControl-rt  Tridium 4.15.6.20  Library of Control Components
  kitControl-ux  Tridium 4.15.6.20  Kit Control UX
  kitControl-wb  Tridium 4.15.6.20  Kit Control Workbench
`
	for _, tt := range []struct{ query, want string }{
		{"KITCONTROL-RT", "MODULE: kitControl-rt\n"},
		{"kitControl", listing},
		{"*-ux", "MODULE: kitControl-ux\n"},
		{"kit*", "3 modules matching 'kit*':\n"},
	} {
		got, misses := runModule(t, ModuleParams{Name: tt.query})
		if !strings.HasPrefix(got, tt.want) || misses != 0 {
			t.Errorf("%s: misses %d, got:\n%s", tt.query, misses, got)
		}
	}
}

// TestModuleMissSaysWhatWasSearched: a miss names the index, its module count
// and the descriptors it could not read, and exits 1.
func TestModuleMissSaysWhatWasSearched(t *testing.T) {
	got, misses := runModule(t, ModuleParams{Name: "nosuch-rt"})
	want := `No match for 'nosuch-rt' (module) in this scope.
  searched: Niagara unregistered (<DIR>) -- module-index.json: 5 modules (1 module.xml unreadable: broken-rt.jar)
  excluded from this scope: jars without META-INF/module.xml; module names are matched exactly, in any letter case, by part name or as a glob
  not proof of absence; try: type 'nosuch-rt', search 'nosuch-rt', find 'nosuch-rt'
`
	if got != want || misses != 1 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

func TestModuleNeedsItsIndex(t *testing.T) {
	err := (&Env{BaseDir: t.TempDir(), Out: &bytes.Buffer{}}).Module(ModuleParams{Name: "baja"})
	var missing *MissingIndexError
	if !errors.As(err, &missing) || missing.Name != "module-index.json" {
		t.Errorf("err = %v", err)
	}
}
