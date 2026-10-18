package classrank

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// ClassTables keeps `data.get("_meta") or {}` only when it is a non-empty
// object: every falsy value is the empty dict (nil). A truthy non-object
// _meta makes Python's staleness check raise; it is read as absent here.
func TestClassTablesMeta(t *testing.T) {
	for doc, kept := range map[string]bool{
		`null`: false, `false`: false, `""`: false, `0`: false, `-0.0`: false, `1e-400`: false, `[]`: false, `{}`: false,
		`"x"`: false, `2`: false, `NaN`: false, `[0]`: false,
		`{"a": null}`: true,
	} {
		v, err := index.Decode([]byte(`{"classes": {}, "_meta": ` + doc + `}`))
		if err != nil {
			t.Fatal(err)
		}
		_, meta, ok := ClassTables(v.(*index.Object))
		if !ok || (meta != nil) != kept {
			t.Errorf("_meta %s: ok %v, meta kept %v, want %v", doc, ok, meta != nil, kept)
		}
	}
}
