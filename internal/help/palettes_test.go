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
}

// TestPaletteMiss: an unknown type is a miss that names palette-index.json,
// its palette count and its unreadable palettes.
func TestPaletteMiss(t *testing.T) {
	for _, p := range []PaletteParams{{Query: "NoSuchThing"}} {
		got, misses := runPalette(t, p)
		if misses != 1 || !strings.Contains(got, "palette-index.json: 2 palettes (1 unreadable: broken-rt.jar!module.palette)") {
			t.Errorf("%+v: misses %d, got:\n%s", p, misses, got)
		}
	}
}
