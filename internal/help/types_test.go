package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runType runs `type` on the module fixture root plus a class index holding
// BBooleanLatch only, so one type has javadoc and the others do not.
func runType(t *testing.T, p TypeParams) (string, int) {
	t.Helper()
	root := moduleRoot(t)
	ci := `{"BBooleanLatch": {"path": "com/tridium/kitControl/util/BBooleanLatch.html", "package": "com.tridium.kitControl.util", "type": "class", "module": "kitControl-rt"}}`
	if err := os.WriteFile(filepath.Join(root, "indexes", "class-index.json"), []byte(ci), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	e := &Env{BaseDir: root, Out: &out}
	if err := e.Type(p); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(out.String(), root, "<DIR>"), e.misses
}

func TestTypeWithItsAgents(t *testing.T) {
	got, misses := runType(t, TypeParams{Name: "kitControl:BooleanLatch"})
	want := `TYPE: kitControl:BooleanLatch
MODULE: kitControl-rt (kitControl-rt.jar)
CLASS: com.tridium.kitControl.util.BBooleanLatch
DOCS: class-index.json has com.tridium.kitControl.util.BBooleanLatch (nh class BBooleanLatch)
SOURCE: not in source-index.json

AGENTS ON THIS TYPE (2):
  kitControl:LatchTypeExt  com.tridium.kitControl.ux.BLatchTypeExt  (kitControl-ux)
  kitControl:LatchView  com.tridium.kitControl.wb.BLatchView  (kitControl-wb)
    app webShell; requires permissions r
`
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestTypeThatIsAnAgent: a view type says which types it is an agent on, and a
// class without javadoc says so.
func TestTypeThatIsAnAgent(t *testing.T) {
	got, _ := runType(t, TypeParams{Name: "LatchView"})
	for _, want := range []string{
		"TYPE: kitControl:LatchView\n",
		"DOCS: not in class-index.json (no javadoc page for BLatchView)\n",
		"AGENTS ON THIS TYPE (0):\n",
		"AGENT ON (1):\n    on: kitControl:BooleanLatch\n    app webShell; requires permissions r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
}

func TestTypeResolution(t *testing.T) {
	for _, tt := range []struct {
		query, want string
		misses      int
	}{
		{"booleanlatch", "TYPE: kitControl:BooleanLatch\n", 0},
		{"KITCONTROL:booleanLatch", "TYPE: kitControl:BooleanLatch\n", 0},
		{"*Latch*", "4 types matching '*Latch*':\n\n" +
			"  kitControl:BooleanLatch  com.tridium.kitControl.util.BBooleanLatch  (kitControl-rt)\n" +
			"  kitControl:NumericLatch  com.tridium.kitControl.util.BNumericLatch  (kitControl-rt)\n" +
			"  kitControl:LatchTypeExt  com.tridium.kitControl.ux.BLatchTypeExt  (kitControl-ux)\n" +
			"  kitControl:LatchView  com.tridium.kitControl.wb.BLatchView  (kitControl-wb)\n", 0},
		{"control:BooleanLatch", "No match for 'control:BooleanLatch' (type) in this scope.\n" +
			"  searched: Niagara unregistered (<DIR>) -- module-index.json: 5 modules (1 module.xml unreadable: broken-rt.jar)\n" +
			"  excluded from this scope: types of jars without META-INF/module.xml; a type is matched as module:Type " +
			"or Type, exactly, in any letter case or as a glob\n" +
			"  not proof of absence; try: class 'control:BooleanLatch', search 'control:BooleanLatch', find 'control:BooleanLatch'\n", 1},
	} {
		got, misses := runType(t, TypeParams{Name: tt.query})
		if !strings.HasPrefix(got, tt.want) || misses != tt.misses {
			t.Errorf("%s: misses %d, got:\n%s\nwant prefix:\n%s", tt.query, misses, got, tt.want)
		}
	}
}

func TestTypeJSON(t *testing.T) {
	got, _ := runType(t, TypeParams{Name: "kitControl:BooleanLatch", JSON: true})
	for _, want := range []string{`"type": "kitControl:BooleanLatch"`, `"module": "kitControl-rt"`,
		`"class_index": true`, `"source_index": false`, `"agent": "kitControl:LatchView"`} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %s:\n%s", want, got)
		}
	}
}
