package fnmatch

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The fixtures are frozen output of the retired testdata/gen.py (Python 3.15 fnmatch).

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
		var pat string
		var hits []string
		var misses int
		for i, v := range []any{&pat, &hits, &misses} {
			if err := json.Unmarshal(c[i], v); err != nil {
				t.Fatal(err)
			}
		}
		var got []string
		n := 0
		for _, name := range f.Names {
			if FnmatchCase(name, pat) {
				got = append(got, name)
			} else {
				n++
			}
			if Fnmatch(name, pat) != FnmatchCase(name, pat) {
				t.Errorf("Fnmatch(%q, %q) differs from FnmatchCase on POSIX", name, pat)
			}
		}
		if strings.Join(got, "\x00") != strings.Join(hits, "\x00") {
			t.Errorf("pattern %q matches %q, want %q", pat, got, hits)
		}
		if n != misses {
			t.Errorf("pattern %q: %d names do not match, want %d", pat, n, misses)
		}
	}
}
