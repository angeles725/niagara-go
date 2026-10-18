package inventory

import (
	"path/filepath"
	"strings"
	"testing"
)

// bool(json.load(f)) of a whole index file for every value class, as CPython
// answers (stats skips a falsy index).
func TestSummaryTruthyMatchesPython(t *testing.T) {
	huge := "1" + strings.Repeat("0", 400)
	path := filepath.Join(t.TempDir(), "x.json")
	for doc, want := range map[string]bool{
		`null`: false, `false`: false, `""`: false, `0`: false, `-0`: false, `0.0`: false, `-0.0`: false,
		`1e-400`: false, `[]`: false, `{}`: false,
		`true`: true, `"x"`: true, `"0"`: true, `2`: true, `-1`: true, `5e-324`: true, `1e400`: true,
		`NaN`: true, `-Infinity`: true, huge: true, `[0]`: true, `{"a": null}`: true,
	} {
		write(t, path, doc)
		s := summarize(path, true)
		if !s.Loaded || s.Truthy != want {
			t.Errorf("summarize(%.20s): loaded %v, truthy %v; want truthy %v", doc, s.Loaded, s.Truthy, want)
		}
	}
}
