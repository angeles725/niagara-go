package blocksearch

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestWindowsPathlib pins the Windows spelling of the pathlib helpers (CPython 3.13
// WindowsPath: every "/" becomes "\", repeated separators and "." parts drop, the drive
// stays), computed on any host.
func TestWindowsPathlib(t *testing.T) {
	for in, want := range map[string]string{
		`a//b/./c/`: `a\b\c`, `./a`: `a`, `.`: `.`, ``: `.`, `/x/../y`: `\x\..\y`,
		`C:/r/docs/`: `C:\r\docs`, `C:`: `C:`, `C:/`: `C:\`, `C:rel/x`: `C:rel\x`,
		`\\srv\share\x/y`: `\\srv\share\x\y`,
	} {
		if got := pyPath(in, true); got != want {
			t.Errorf("pyPath(%q, windows) = %q, want %q", in, got, want)
		}
	}
	joins := []struct{ a, b, want string }{
		{".", "docs", "docs"},
		{"C:/r", "docs", `C:\r\docs`},
		{`C:\r\`, "docs/sub/x.md", `C:\r\docs\sub\x.md`},
		{"testdata/corpus", "CATALOG.md", `testdata\corpus\CATALOG.md`},
	}
	for _, j := range joins {
		if got := pyJoin(j.a, j.b, true); got != j.want {
			t.Errorf("pyJoin(%q, %q, windows) = %q, want %q", j.a, j.b, got, j.want)
		}
	}
	if got := relTo(`C:\r\docs\a.md`, `C:\r`, true); got != `docs\a.md` {
		t.Errorf("relTo windows = %q", got)
	}
	// WindowsPath orders by the case-folded components.
	if !pathLess(`docs\a\x.md`, `docs\a-b.md`, true) || !pathLess(`docs\a.md`, `docs\B.md`, true) {
		t.Error("WindowsPath order compares case-folded components")
	}
}

// TestPathlibFollowsTheHost: corpus paths are pathlib's, so on Windows a docs id is
// "docs\\guide.md" as the Python tool prints it.
func TestPathlibFollowsTheHost(t *testing.T) {
	c := New("testdata/corpus", "")
	want := filepath.FromSlash("testdata/corpus/docs")
	if c.Docs != want {
		t.Errorf("Docs = %q, want %q", c.Docs, want)
	}
	if got := RelTo(PyJoin(c.Docs, "a.md"), c.Root); got != filepath.FromSlash("docs/a.md") {
		t.Errorf("RelTo = %q", got)
	}
	if runtime.GOOS != "windows" && PyPath("a//b/") != "a/b" {
		t.Error("PurePosixPath spelling off Windows")
	}
}
