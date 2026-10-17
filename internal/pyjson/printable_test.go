package pyjson

import (
	"encoding/json"
	"os"
	"testing"
	"unicode"
)

// pyUnidataVersion is unicodedata.unidata_version of the Python 3.15 that
// froze testdata/py-isprintable.json, the [first, last] code-point ranges
// where chr(c).isprintable() is True (all of U+0000..U+10FFFF).
const pyUnidataVersion = "17.0.0"

// TestIsPyPrintableMatchesPython pins isPyPrintable (Go's unicode.IsPrint)
// to Python's str.isprintable over every code point. Both use Unicode 17.0.0
// today; a Go or Python upgrade that moves either table fails here and the
// fixture must be refrozen (or isPyPrintable given an explicit table).
func TestIsPyPrintableMatchesPython(t *testing.T) {
	if unicode.Version != pyUnidataVersion {
		t.Errorf("Go unicode.Version %s, Python unidata_version %s: refreeze testdata/py-isprintable.json",
			unicode.Version, pyUnidataVersion)
	}
	raw, err := os.ReadFile("testdata/py-isprintable.json")
	if err != nil {
		t.Fatal(err)
	}
	var ranges [][2]rune
	if err := json.Unmarshal(raw, &ranges); err != nil {
		t.Fatal(err)
	}
	py := make([]bool, unicode.MaxRune+1)
	for _, r := range ranges {
		for c := r[0]; c <= r[1]; c++ {
			py[c] = true
		}
	}
	diffs := 0
	for c := rune(0); c <= unicode.MaxRune; c++ {
		if got := isPyPrintable(c); got != py[c] {
			if diffs++; diffs <= 20 {
				t.Errorf("U+%04X: isPyPrintable %v, Python isprintable %v", c, got, py[c])
			}
		}
	}
	if diffs > 20 {
		t.Errorf("%d code points disagree in total", diffs)
	}
}
