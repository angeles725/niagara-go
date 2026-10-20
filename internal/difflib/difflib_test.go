package difflib

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The fixtures are frozen output of the retired testdata/gen.py (Python 3.15 difflib).

type fixtures struct {
	Matcher [][]json.RawMessage `json:"matcher"`
	Close   [][]json.RawMessage `json:"close"`
}

func load(t *testing.T) fixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
}

func fromBits(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	var h string
	decode(t, raw, &h)
	u, err := strconv.ParseUint(h, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

type result struct {
	longest  Match
	blocks   []Match
	r, q, rq float64
}

func run[T comparable](junk func(T) bool, a, b []T, autojunk bool) result {
	m := New(junk, a, b, autojunk)
	return result{m.FindLongestMatch(0, len(a), 0, len(b)), m.GetMatchingBlocks(), m.Ratio(), m.QuickRatio(),
		m.RealQuickRatio()}
}

func TestSequenceMatcher(t *testing.T) {
	for i, c := range load(t).Matcher {
		var kind, junk string
		var autojunk bool
		decode(t, c[0], &kind)
		decode(t, c[1], &junk)
		decode(t, c[2], &autojunk)
		var got result
		if kind == "str" {
			var a, b string
			decode(t, c[3], &a)
			decode(t, c[4], &b)
			var isJunk func(rune) bool
			if junk == "space" {
				isJunk = func(r rune) bool { return r == ' ' || r == '\t' }
			}
			got = run(isJunk, Runes(a), Runes(b), autojunk)
		} else {
			var a, b []string
			decode(t, c[3], &a)
			decode(t, c[4], &b)
			var isJunk func(string) bool
			if junk == "blank" {
				isJunk = func(s string) bool { return pycompat.Strip(s) == "" }
			}
			got = run(isJunk, a, b, autojunk)
		}
		var longest [3]int
		var blocks [][3]int
		decode(t, c[5], &longest)
		decode(t, c[6], &blocks)
		if got.longest != (Match{longest[0], longest[1], longest[2]}) {
			t.Errorf("case %d: longest %v, want %v", i, got.longest, longest)
		}
		if len(got.blocks) != len(blocks) {
			t.Errorf("case %d: blocks %v, want %v", i, got.blocks, blocks)
		} else {
			for k, x := range blocks {
				if got.blocks[k] != (Match{x[0], x[1], x[2]}) {
					t.Errorf("case %d: blocks %v, want %v", i, got.blocks, blocks)
					break
				}
			}
		}
		for k, v := range []float64{got.r, got.q, got.rq} {
			if want := fromBits(t, c[7+k]); math.Float64bits(v) != math.Float64bits(want) {
				t.Errorf("case %d: ratio #%d = %v, want %v", i, k, v, want)
			}
		}
	}
}

func TestGetCloseMatches(t *testing.T) {
	for _, c := range load(t).Close {
		var word string
		var names []string
		var n int
		var cutoff float64
		var want *[]string
		decode(t, c[0], &word)
		decode(t, c[1], &names)
		decode(t, c[2], &n)
		decode(t, c[3], &cutoff)
		decode(t, c[4], &want)
		got, err := GetCloseMatches(word, names, n, cutoff)
		switch {
		case want == nil && err == nil:
			t.Errorf("GetCloseMatches(%q, %d, %v) = %q, want ValueError", word, n, cutoff, got)
		case want != nil && (err != nil || strings.Join(got, "|") != strings.Join(*want, "|")):
			t.Errorf("GetCloseMatches(%q, %d, %v) = %q, %v; want %q", word, n, cutoff, got, err, *want)
		}
	}
}
