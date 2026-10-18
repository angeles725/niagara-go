package core

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// bool(e.get("k")) for every value class json.loads can give, as CPython answers.
func TestTruthyMatchesPython(t *testing.T) {
	huge := "1" + strings.Repeat("0", 400)
	for doc, want := range map[string]bool{
		`null`: false, `false`: false, `""`: false, `0`: false, `-0`: false, `0.0`: false, `-0.0`: false,
		`1e-400`: false, `[]`: false, `{}`: false,
		`true`: true, `"x"`: true, `"0"`: true, `2`: true, `-1`: true, `5e-324`: true, `1e400`: true,
		`NaN`: true, `-Infinity`: true, huge: true, `[0]`: true, `{"a": null}`: true,
	} {
		v, err := index.Decode([]byte(`{"k": ` + doc + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := Truthy(v.(*index.Object), "k"); got != want {
			t.Errorf("Truthy(e, k) with k=%.20s = %v, want %v", doc, got, want)
		}
	}
	if Truthy(index.NewObject(), "k") {
		t.Error("a missing key is truthy")
	}
}
