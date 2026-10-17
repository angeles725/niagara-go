package pyjson

import (
	"errors"
	"strings"
	"testing"
)

func nested(open, leaf, close string, n int) []rune {
	return []rune(strings.Repeat(open, n) + leaf + strings.Repeat(close, n))
}

// TestLoadsBoundsNesting: json.loads raises RecursionError on runaway nesting
// (CPython 3.15: "Stack overflow (used N kB) while decoding a JSON array from
// a unicode string" past ~52,100 levels on an 8 MiB stack); Loads stops at
// MaxDepth with a fixed RecursionError instead of overflowing Go's stack.
func TestLoadsBoundsNesting(t *testing.T) {
	for _, tc := range []struct {
		name, open, leaf, close, kind string
	}{
		{"array", "[", "", "]", "array"},
		{"object", `{"a":`, "1", "}", "object"},
	} {
		if _, err := Loads(nested(tc.open, tc.leaf, tc.close, MaxDepth)); err != nil {
			t.Errorf("%s at MaxDepth: %v", tc.name, err)
		}
		for _, n := range []int{MaxDepth + 1, 10_000_000} {
			_, err := Loads(nested(tc.open, tc.leaf, tc.close, n))
			var re *RecursionError
			want := "maximum recursion depth exceeded while decoding a JSON " + tc.kind + " from a unicode string"
			if !errors.As(err, &re) || err.Error() != want {
				t.Errorf("%s depth %d: %v, want RecursionError %q", tc.name, n, err, want)
			}
		}
	}
}
