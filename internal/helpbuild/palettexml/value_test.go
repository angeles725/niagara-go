package palettexml

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestValueIsTheIndexTree: folders nest their children; an entry carries its
// properties (type and value only when set) and the types below it, and an
// entry without either has only its name and type.
func TestValueIsTheIndexTree(t *testing.T) {
	p, err := Parse([]byte(kitControlPalette))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"folder": "ControlPalette", "type": "baja:UnrestrictedFolder", "children": [` +
		`{"folder": "Points", "type": "baja:Folder", "children": [{"name": "BooleanPoint", "type": "control:BooleanPoint"}]}, ` +
		`{"folder": "Trigger", "type": "baja:UnrestrictedFolder", "children": [{"name": "Daily", "type": "control:TimeTrigger", ` +
		`"properties": [{"name": "triggerMode", "type": "control:DailyTriggerMode"}, {"name": "enabled", "value": "true"}], ` +
		`"contains": ["control:DailyTriggerMode", "kitControl:Weekdays"]}]}]}, ` +
		`{"folder": "Latch Blocks", "type": "baja:UnrestrictedFolder", "children": [{"name": "BooleanLatch", "type": "kitControl:BooleanLatch"}]}]`
	v := p.Value()
	if got := pyjson.Dumps(v, pyjson.DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "}); got != want {
		t.Errorf("value:\n%s\nwant:\n%s", got, want)
	}
}

// TestWalkAndCounts: Walk visits folders and entries with their paths (an
// entry's properties are not nodes) and Counts totals them, the root excluded.
func TestWalkAndCounts(t *testing.T) {
	p, err := Parse([]byte(kitControlPalette))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Counts(); got != (Counts{Folders: 4, Entries: 3, Properties: 2}) {
		t.Errorf("counts = %+v", got)
	}
	var paths []string
	p.Walk(func(path []string, n *Node) { paths = append(paths, strings.Join(path, "/")) })
	want := "ControlPalette ControlPalette/Points ControlPalette/Points/BooleanPoint ControlPalette/Trigger " +
		"ControlPalette/Trigger/Daily Latch Blocks Latch Blocks/BooleanLatch"
	if got := strings.Join(paths, " "); got != want {
		t.Errorf("walk = %s", got)
	}
}
