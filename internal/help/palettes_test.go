package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runPalette runs `palette` on a root holding the palette-index.json fixture.
func runPalette(t *testing.T, p PaletteParams) (string, int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "palette-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "indexes", "palette-index.json"), string(data))
	var out bytes.Buffer
	e := &Env{BaseDir: root, Out: &out}
	if err := e.Palette(p); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(out.String(), root, "<DIR>"), e.misses
}

// TestPaletteFindsWhereATypeIs: a type is found by Type or module:Type as an
// entry and inside the entries that hold it; a name or folder matches too;
// letter case and globs are later rounds, as for nh type.
func TestPaletteFindsWhereATypeIs(t *testing.T) {
	for _, tt := range []struct {
		name, query, want string
	}{
		{"type", "BooleanLatch", `PALETTES with 'BooleanLatch' (2):

  kitControl-rt  module.palette  Logic/Latches/BooleanLatch  kitControl:BooleanLatch
  kitControl-rt  module.palette  Logic/LatchBank  kitControl:LatchBank  (holds kitControl:BooleanLatch)
`},
		{"qualified", "control:DailyTriggerMode", `PALETTES with 'control:DailyTriggerMode' (1):

  control-rt  module.palette  Points/Daily Trigger  control:TimeTrigger  (holds control:DailyTriggerMode)
`},
		{"display name", "daily trigger", `PALETTES with 'daily trigger' (1):

  control-rt  module.palette  Points/Daily Trigger  control:TimeTrigger
`},
		{"folder", "Latches", `PALETTES with 'Latches' (1):

  kitControl-rt  module.palette  Logic/Latches/  folder (2 items)
`},
		{"glob", "*Latch", `PALETTES with '*Latch' (3):

  kitControl-rt  module.palette  Logic/Latches/BooleanLatch  kitControl:BooleanLatch
  kitControl-rt  module.palette  Logic/Latches/NumericLatch  kitControl:NumericLatch
  kitControl-rt  module.palette  Logic/LatchBank  kitControl:LatchBank  (holds kitControl:BooleanLatch)
`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, misses := runPalette(t, PaletteParams{Query: tt.query})
			if got != tt.want || misses != 0 {
				t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, tt.want)
			}
		})
	}
}

// TestPaletteModuleTree: --module prints each palette of the module as a
// tree, an entry with its type, properties and the types it holds.
func TestPaletteModuleTree(t *testing.T) {
	got, misses := runPalette(t, PaletteParams{Query: "control-rt", Module: true})
	want := `PALETTE: control-rt module.palette (control-rt.jar; 1 folders, 2 entries)
  Points/
    BooleanPoint  control:BooleanPoint
    Daily Trigger  control:TimeTrigger
      triggerMode: control:DailyTriggerMode
      enabled = true
      holds: control:DailyTriggerMode
`
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

func TestPaletteJSON(t *testing.T) {
	got, _ := runPalette(t, PaletteParams{Query: "NumericLatch", JSON: true})
	want := `{
  "query": "NumericLatch",
  "hits": [
    {
      "module": "kitControl-rt",
      "palette": "module.palette",
      "path": "Logic/Latches/NumericLatch",
      "type": "kitControl:NumericLatch",
      "kind": "entry"
    }
  ]
}
`
	if got != want {
		t.Errorf("got:\n%s", got)
	}
	got, _ = runPalette(t, PaletteParams{Query: "kitControl-rt", Module: true, JSON: true})
	if !strings.HasPrefix(got, "{\n  \"module\": \"kitControl-rt\",\n  \"palettes\": [\n    {\n      \"module\": \"kitControl-rt\",") {
		t.Errorf("module json:\n%s", got)
	}
}

// TestPaletteMiss: an unknown type and a module without palettes are misses
// that name palette-index.json, its palette count and its unreadable palettes.
func TestPaletteMiss(t *testing.T) {
	for _, p := range []PaletteParams{{Query: "NoSuchThing"}, {Query: "baja", Module: true}} {
		got, misses := runPalette(t, p)
		if misses != 1 || !strings.Contains(got, "palette-index.json: 2 palettes (1 unreadable: broken-rt.jar!module.palette)") {
			t.Errorf("%+v: misses %d, got:\n%s", p, misses, got)
		}
	}
}

// TestPaletteCLI: palette takes a query, --module and --json; a miss exits 1.
func TestPaletteCLI(t *testing.T) {
	dir := repo(t, `{"default": "1.0.0.1", "versions": {"1.0.0.1": {"install_dir": ".", "data_root": "."}}}`, ".")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "palette-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "indexes", "palette-index.json"), string(data))
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"palette", "BooleanPoint"}, 0, "PALETTES with 'BooleanPoint' (1):\n"},
		{[]string{"palette", "--module", "control-rt"}, 0, "PALETTE: control-rt module.palette"},
		{[]string{"palette", "BooleanPoint", "--json"}, 0, "{\n  \"query\": \"BooleanPoint\","},
		{[]string{"palette", "Nope"}, 1, ""},
	} {
		out, errs, code := run(t, dir, tt.args...)
		if code != tt.code || !strings.HasPrefix(out, tt.out) {
			t.Errorf("%v: code %d stdout %q stderr %q", tt.args, code, out, errs)
		}
	}
}
