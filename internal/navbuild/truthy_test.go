package navbuild

import (
	"strings"
	"testing"
)

// bool(e.get("outer_class")) of a class index entry for every value class
// json.loads can give, as CPython answers.
func TestOuterMatchesPython(t *testing.T) {
	huge := "1" + strings.Repeat("0", 400)
	for doc, want := range map[string]bool{
		`null`: false, `false`: false, `""`: false, `0`: false, `-0`: false, `0.0`: false, `-0.0`: false,
		`1e-400`: false, `[]`: false, `{}`: false,
		`true`: true, `"x"`: true, `"0"`: true, `2`: true, `-1`: true, `5e-324`: true, `1e400`: true,
		`NaN`: true, `-Infinity`: true, huge: true, `[0]`: true, `{"a": null}`: true,
	} {
		base := newBase(t)
		mustWrite(t, IndexPath(base, ClassIndexFile), `{"classes": {"A": [{"outer_class": `+doc+`, "path": "a/A.java"}]}}`)
		got, err := readClassIndex(base)
		if err != nil || len(got) != 1 {
			t.Fatalf("outer_class %.20s: %d entries, err %v", doc, len(got), err)
		}
		if got[0].Outer != want {
			t.Errorf("outer_class %.20s: Outer = %v, want %v", doc, got[0].Outer, want)
		}
	}
}
