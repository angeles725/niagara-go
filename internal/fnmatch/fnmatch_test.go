package fnmatch

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The fixtures are written by testdata/gen.py from Python 3.15 fnmatch.

type fixtures struct {
	Names []string            `json:"names"`
	Cases [][]json.RawMessage `json:"cases"`
}

func TestMatchesPython(t *testing.T) {
	data, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.Cases {
		var pat, translated string
		var hits []string
		var misses int
		for i, v := range []any{&pat, &translated, &hits, &misses} {
			if err := json.Unmarshal(c[i], v); err != nil {
				t.Fatal(err)
			}
		}
		if got := Translate(pat); got != translated {
			t.Errorf("Translate(%q) = %q, want %q", pat, got, translated)
		}
		if got := Filter(f.Names, pat); strings.Join(got, "\x00") != strings.Join(hits, "\x00") {
			t.Errorf("Filter(%q) = %q, want %q", pat, got, hits)
		}
		n := 0
		for _, name := range f.Names {
			if !FnmatchCase(name, pat) {
				n++
			}
			if Fnmatch(name, pat) != FnmatchCase(name, pat) {
				t.Errorf("Fnmatch(%q, %q) differs from FnmatchCase on POSIX", name, pat)
			}
		}
		if n != misses {
			t.Errorf("pattern %q: %d names do not match, want %d", pat, n, misses)
		}
	}
}
