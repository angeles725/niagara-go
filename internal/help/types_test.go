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

// TestTypeNamesItsPalettes: with palette-index.json beside the module index,
// a type that sits in a palette names where (and the entries holding it); a
// type in no palette prints no PALETTES line; --json lists them.
func TestTypeNamesItsPalettes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "palette-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := moduleRoot(t)
	write(t, filepath.Join(root, "indexes", "palette-index.json"), string(data))
	run := func(p TypeParams) string {
		var out bytes.Buffer
		if err := (&Env{BaseDir: root, Out: &out}).Type(p); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	want := "SOURCE: not in source-index.json\nPALETTES: kitControl-rt module.palette Logic/Latches/BooleanLatch; " +
		"kitControl-rt module.palette Logic/LatchBank (holds it)  (nh palette kitControl:BooleanLatch)\n"
	if got := run(TypeParams{Name: "kitControl:BooleanLatch"}); !strings.Contains(got, want) {
		t.Errorf("lacks %q:\n%s", want, got)
	}
	if got := run(TypeParams{Name: "LatchView"}); strings.Contains(got, "PALETTES") {
		t.Errorf("a type in no palette:\n%s", got)
	}
	want = `  "palettes": [
    {
      "module": "kitControl-rt",
      "palette": "module.palette",
      "path": "Logic/Latches/BooleanLatch",
      "holds": false
    },`
	if got := run(TypeParams{Name: "kitControl:BooleanLatch", JSON: true}); !strings.Contains(got, want) {
		t.Errorf("json lacks %q:\n%s", want, got)
	}
}

// TestTypeNamesItsTags: with tag-index.json beside the module index, a type
// a rule, tag group or tag condition names (isType module:Type, exactly)
// lists the dictionary names it is given; another type prints no TAGS line;
// --json lists them.
func TestTypeNamesItsTags(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "tag-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := moduleRoot(t)
	write(t, filepath.Join(root, "indexes", "tag-index.json"),
		strings.Replace(string(data), "isType kitControl:Ahu", "isType kitControl:BooleanLatch", 1))
	run := func(p TypeParams) string {
		var out bytes.Buffer
		if err := (&Env{BaseDir: root, Out: &out}).Type(p); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	want := "TAGS (isType kitControl:BooleanLatch): hs:ahu, hs:ahuEquip  (nh tag <name>)\n"
	if got := run(TypeParams{Name: "kitControl:BooleanLatch"}); !strings.Contains(got, want) {
		t.Errorf("lacks %q:\n%s", want, got)
	}
	if got := run(TypeParams{Name: "LatchView"}); strings.Contains(got, "TAGS") {
		t.Errorf("a type no condition names:\n%s", got)
	}
	want = "  \"tags\": [\n    \"hs:ahu\",\n    \"hs:ahuEquip\"\n  ]"
	if got := run(TypeParams{Name: "kitControl:BooleanLatch", JSON: true}); !strings.Contains(got, want) {
		t.Errorf("json lacks %q:\n%s", want, got)
	}
}

// TestNamesType: isType module:Type is matched as a whole type name.
func TestNamesType(t *testing.T) {
	for _, tt := range []struct {
		cond string
		want bool
	}{
		{"isType control:ControlPoint", true},
		{"(filter hs:site and isType control:ControlPoint)", true},
		{"isType control:ControlPointExt", false},
		{"isType control:ControlPointExt or isType control:ControlPoint", true},
		{"hasTags point", false},
	} {
		if got := namesType(tt.cond, "control:ControlPoint"); got != tt.want {
			t.Errorf("%q: %v", tt.cond, got)
		}
	}
}
