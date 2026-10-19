package pypath

import (
	"path/filepath"
	"testing"
)

// TestPurePosixPath pins str(PurePosixPath(p)) (CPython 3.13), the same on every host.
func TestPurePosixPath(t *testing.T) {
	for in, want := range map[string]string{"a//b/./c/": "a/b/c", "./x": "x", "/a/../b": "/a/../b", "": ".",
		"//srv/x": "//srv/x", "///x": "/x", "missing/corpus-nav.py": "missing/corpus-nav.py"} {
		if got := PurePosixPath(in); got != want {
			t.Errorf("PurePosixPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPosixStr pins the off-Windows spelling of Str, computed on any host. "//srv/x"
// folds to "/srv/x", where CPython keeps "//srv/x" (the package's recorded follow-up).
func TestPosixStr(t *testing.T) {
	for in, want := range map[string]string{
		"a//b/./c/": "a/b/c", "./a": "a", ".": ".", "": ".", "/x/../y": "/x/../y", "//srv/x": "/srv/x",
	} {
		if got := str(in, false); got != want {
			t.Errorf("str(%q, posix) = %q, want %q", in, got, want)
		}
	}
}

// TestWindowsPathlib pins the Windows spelling of the pathlib helpers (CPython 3.13
// WindowsPath: every "/" becomes "\", repeated separators and "." parts drop, the drive
// stays), computed on any host.
func TestWindowsPathlib(t *testing.T) {
	for in, want := range map[string]string{
		`a//b/./c/`: `a\b\c`, `./a`: `a`, `.`: `.`, ``: `.`, `/x/../y`: `\x\..\y`,
		`C:/r/docs/`: `C:\r\docs`, `C:`: `C:`, `C:/`: `C:\`, `C:rel/x`: `C:rel\x`,
		`\\srv\share\x/y`: `\\srv\share\x\y`,
	} {
		if got := str(in, true); got != want {
			t.Errorf("str(%q, windows) = %q, want %q", in, got, want)
		}
	}
	if got := relTo(`C:\r\docs\a.md`, `C:\r`, true); got != `docs\a.md` {
		t.Errorf("relTo windows = %q", got)
	}
	// WindowsPath orders by the case-folded components.
	if !less(`docs\a\x.md`, `docs\a-b.md`, true) || !less(`docs\a.md`, `docs\B.md`, true) {
		t.Error("WindowsPath order compares case-folded components")
	}
	if less(`docs\a.md`, `docs\B.md`, false) {
		t.Error("PurePosixPath order is case-sensitive")
	}
}

// TestJoin pins str(Path(a) / b) for both spellings, computed on any host.
func TestJoin(t *testing.T) {
	for _, tt := range []struct {
		a, b    string
		windows bool
		want    string
	}{
		{".", "docs", true, "docs"},
		{"C:/r", "docs", true, `C:\r\docs`},
		{`C:\r\`, "docs/sub/x.md", true, `C:\r\docs\sub\x.md`},
		{"testdata/corpus", "CATALOG.md", true, `testdata\corpus\CATALOG.md`},
		{".", "a.md", false, "a.md"},
		{"r", "a.md", false, "r/a.md"},
	} {
		if got := join(tt.a, tt.b, tt.windows); got != tt.want {
			t.Errorf("join(%q, %q, %v) = %q, want %q", tt.a, tt.b, tt.windows, got, tt.want)
		}
	}
}

// TestFollowsTheHost: Str, Join, RelTo and Less are pathlib's for the running host, so
// on Windows a docs id is "docs\\guide.md" as the Python tools print it.
func TestFollowsTheHost(t *testing.T) {
	docs := Join(Str("testdata//corpus/"), "docs")
	if want := filepath.FromSlash("testdata/corpus/docs"); docs != want {
		t.Errorf("Join(Str(..), docs) = %q, want %q", docs, want)
	}
	if got := RelTo(Join(docs, "a.md"), Str("testdata/corpus")); got != filepath.FromSlash("docs/a.md") {
		t.Errorf("RelTo = %q", got)
	}
	if !Less(filepath.FromSlash("docs/a/x.md"), filepath.FromSlash("docs/a-b.md")) {
		t.Error("Path order compares components, not strings")
	}
}
