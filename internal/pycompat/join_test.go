package pycompat

import (
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/pypath"
)

// TestJoinFollowsTheHost: Join is ntpath.join on Windows and posixpath.join elsewhere.
func TestJoinFollowsTheHost(t *testing.T) {
	flavour := pypath.PosixJoin
	if runtime.GOOS == "windows" {
		flavour = pypath.NtJoin
	}
	for _, parts := range [][]string{{"a"}, {"a", "b"}, {"a/", "b"}, {"a", "/b", "c"}, {"c:", "d"},
		{`a\`, "b"}, {"", "a"}, {"a", ""}} {
		if got, want := Join(parts[0], parts[1:]...), flavour(parts[0], parts[1:]...); got != want {
			t.Errorf("Join(%q) = %q, want %q", parts, got, want)
		}
	}
}
