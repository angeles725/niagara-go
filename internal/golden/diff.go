package golden

import (
	"bytes"
	"fmt"
	"strings"
)

// Diff reports the first differing line with a little context; enough to
// locate a parity break without pulling in a diff library.
func Diff(want, got []byte) string {
	w := strings.Split(string(want), "\n")
	g := strings.Split(string(got), "\n")
	n := len(w)
	if len(g) > n {
		n = len(g)
	}
	for i := 0; i < n; i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl || (i >= len(w)) != (i >= len(g)) {
			var b bytes.Buffer
			fmt.Fprintf(&b, "line %d (want %d lines, got %d)\n", i+1, len(w), len(g))
			fmt.Fprintf(&b, "  want: %q\n  got:  %q", wl, gl)
			return b.String()
		}
	}
	return "identical lines; byte difference (trailing newline or encoding)"
}
