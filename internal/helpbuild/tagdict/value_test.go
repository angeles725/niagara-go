package tagdict

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestAppendSerializesEveryRecord: each record becomes one tag-index.json
// row under its section, naming the file it came from; empty fields are left
// out.
func TestAppendSerializesEveryRecord(t *testing.T) {
	f := &File{
		Tags: []Tag{{Dict: "hs", Name: "ahu", Kind: "Marker", Doc: "Air Handling Unit", Is: []string{"equip"},
			Supertypes: []string{"equip", "marker"}}, {Dict: "hs", Name: "power", Kind: "Number", Units: "kW",
			Implied: []Field{{"hasTags", "point"}}, Validity: []Field{{"isType", "control:NumericPoint"}}}},
		Protos:     []string{"ahu equip"},
		Groups:     []Group{{Dict: "hs", Name: "g", Tags: []string{"a", "b"}}},
		Relations:  []Relation{{"n", "parent", "tagdictionary:ComponentParentRelation"}},
		Rules:      []Rule{{Dict: "n", Name: "point tags", Condition: "always", Tags: []RuleTag{{Name: "point", Validity: "always"}}}},
		Migrations: []Migration{{From: "ahuRef", FromType: "relation", To: []string{"airRef"}, ToType: "relation"}},
	}
	var lines []string
	f.Append("h.jar!d/x", func(section string, v pyjson.Value) {
		lines = append(lines, section+" "+pyjson.Dumps(v, pyjson.Defaults(-1)))
	})
	want := []string{
		`tags {"dict": "hs", "name": "ahu", "file": "h.jar!d/x", "kind": "Marker", "doc": "Air Handling Unit", "is": ["equip"], "supertypes": ["equip", "marker"]}`,
		`tags {"dict": "hs", "name": "power", "file": "h.jar!d/x", "kind": "Number", "units": "kW", "implied": {"hasTags": "point"}, "validity": {"isType": "control:NumericPoint"}}`,
		`protos {"file": "h.jar!d/x", "proto": "ahu equip"}`,
		`groups {"dict": "hs", "name": "g", "file": "h.jar!d/x", "tags": ["a", "b"]}`,
		`relations {"dict": "n", "name": "parent", "file": "h.jar!d/x", "type": "tagdictionary:ComponentParentRelation"}`,
		`rules {"dict": "n", "name": "point tags", "file": "h.jar!d/x", "condition": "always", "tags": [{"name": "point", "validity": "always"}]}`,
		`migrations {"file": "h.jar!d/x", "from": "ahuRef", "fromType": "relation", "to": ["airRef"], "toType": "relation"}`,
	}
	if got := strings.Join(lines, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("got:\n%s", got)
	}
}
