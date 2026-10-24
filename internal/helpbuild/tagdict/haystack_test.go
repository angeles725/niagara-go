package tagdict

import (
	"reflect"
	"strings"
	"testing"
)

// defsJSON is a trimmed haystack-rt defs.json (Haystack 4, 4.13 to 4.15):
// a grid whose rows are defs, symbols as {"_kind": "symbol"} objects.
const defsJSON = `{"_kind": "grid", "meta": {"ver": "3.0"}, "cols": [{"name": "def"}, {"name": "doc"}],
 "rows": [
  {"def": {"_kind": "symbol", "val": "marker"}, "doc": "Marker labels a dict", "lib": {"_kind": "symbol", "val": "lib:ph"}},
  {"def": {"_kind": "symbol", "val": "val"}, "doc": "Data value"},
  {"def": {"_kind": "symbol", "val": "scalar"}, "doc": "Atomic value", "is": [{"_kind": "symbol", "val": "val"}]},
  {"def": {"_kind": "symbol", "val": "str"}, "doc": "Unicode string", "is": [{"_kind": "symbol", "val": "scalar"}]},
  {"def": {"_kind": "symbol", "val": "entity"}, "doc": "Top-level entity", "is": [{"_kind": "symbol", "val": "marker"}]},
  {"def": {"_kind": "symbol", "val": "equip"}, "doc": "Equipment asset", "is": [{"_kind": "symbol", "val": "entity"}]},
  {"def": {"_kind": "symbol", "val": "ahu"}, "doc": "Air Handling Unit", "is": [{"_kind": "symbol", "val": "equip"}],
   "lib": {"_kind": "symbol", "val": "lib:phIoT"}},
  {"def": {"_kind": "symbol", "val": "tz"}, "doc": "Timezone", "is": [{"_kind": "symbol", "val": "str"}],
   "tagOn": [{"_kind": "symbol", "val": "site"}, {"_kind": "symbol", "val": "point"}],
   "of": {"_kind": "symbol", "val": "site"}}
 ]}`

func TestParseDefsResolvesKindsAndSupertypes(t *testing.T) {
	f, err := Read("com/tridium/haystack/data/haystack-4-defs/defs/defs.json", []byte(defsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != "defs" || len(f.Tags) != 8 {
		t.Fatalf("kind %q, %d tags", f.Kind, len(f.Tags))
	}
	ahu, tz := f.Tags[6], f.Tags[7]
	want := Tag{Dict: "hs", Name: "ahu", Kind: "Marker", Doc: "Air Handling Unit", Is: []string{"equip"},
		Supertypes: []string{"equip", "entity", "marker"}, Lib: "lib:phIoT"}
	if !reflect.DeepEqual(ahu, want) {
		t.Errorf("ahu = %+v", ahu)
	}
	if tz.Kind != "Str" || !reflect.DeepEqual(tz.TagOn, []string{"site", "point"}) || tz.Of != "site" {
		t.Errorf("tz = %+v", tz)
	}
	if f.Tags[1].Kind != "" { // val is no kind and has none above it
		t.Errorf("val kind = %q", f.Tags[1].Kind)
	}
}

func TestParseProtosAndRules(t *testing.T) {
	protos := `{"_kind": "grid", "cols": [{"name": "proto"}, {"name": "air"}],
	 "rows": [{"proto": "air temp sensor point", "air": {"_kind": "marker"}}, {"proto": "ahu equip"}]}`
	f, err := Read("x/protos/protos.json", []byte(protos))
	if err != nil || f.Kind != "protos" || !reflect.DeepEqual(f.Protos, []string{"air temp sensor point", "ahu equip"}) {
		t.Fatalf("%+v %v", f, err)
	}
	rules := `{"tagRules": [{"name": "cur", "condition": "IS_TYPE", "isType": "control:ControlPoint"},
	 {"name": "equipRef", "condition": "ALWAYS", "tagType": "haystack:EquipRelation"}], "libPHIncludes": ["min"]}`
	f, err = Read("x/HS4toN4.json", []byte(rules))
	if err != nil || f.Kind != "rules" || len(f.Rules) != 2 {
		t.Fatalf("%+v %v", f, err)
	}
	want := Rule{Dict: "hs", Name: "equipRef", Condition: "always", Tags: []RuleTag{{Name: "equipRef", Type: "haystack:EquipRelation"}}}
	if !reflect.DeepEqual(f.Rules[1], want) || f.Rules[0].Condition != "isType control:ControlPoint" {
		t.Errorf("rules = %+v", f.Rules)
	}
}

func TestReadFailsClosedOnJSON(t *testing.T) {
	for name, data := range map[string]string{
		"not JSON":       `{"_kind": "grid"`,
		"unknown grid":   `{"_kind": "grid", "cols": [{"name": "id"}], "rows": []}`,
		"unknown object": `{"rows": []}`,
		"def row no def": `{"_kind": "grid", "cols": [{"name": "def"}], "rows": [{"doc": "x"}]}`,
		"rule no name":   `{"tagRules": [{"condition": "ALWAYS"}]}`,
		"proto no proto": `{"_kind": "grid", "cols": [{"name": "proto"}], "rows": [{"air": {"_kind": "marker"}}]}`,
	} {
		if f, err := Read("a.json", []byte(data)); err == nil {
			t.Errorf("%s: read as %+v", name, f)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: empty error", name)
		}
	}
}
