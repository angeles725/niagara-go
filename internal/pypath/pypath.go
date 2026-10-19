// Package pypath is Python's pathlib string forms: str(PurePosixPath(p)) on every host,
// and str(pathlib.Path(p)) with Path(a) / b, relative_to and ordering spelled the host's
// way (PurePosixPath off Windows; WindowsPath on Windows, where every "/" becomes "\",
// with drives and UNC shares). It is a non-printing library.
//
// Owner: Integrante2 (session 5, WU8). It is one package for what corpusnav/blocksearch,
// blockfiles and searchdoor each emulated: blocksearch imports blockfiles, so the shared
// code cannot live in blocksearch, and pycompat (Python 3.15 behaviour in general) has
// another owner.
//
// Follow-up: POSIX Str keeps today's '//'→'/' (corpusnav/corpus_test.go:23 pins it);
// CPython keeps '//'.
package pypath

import (
	"runtime"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

const hostWindows = runtime.GOOS == "windows"

// PurePosixPath is str(pathlib.PurePosixPath(p)) on every host: repeated slashes and "."
// parts dropped, no trailing slash, ".." kept, exactly two leading slashes kept.
func PurePosixPath(p string) string {
	if p == "" {
		return "."
	}
	lead := ""
	switch {
	case strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///"):
		lead = "//"
	case strings.HasPrefix(p, "/"):
		lead = "/"
	}
	var parts []string
	for _, part := range strings.Split(p, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		if lead != "" {
			return lead
		}
		return "."
	}
	return lead + strings.Join(parts, "/")
}

// Str is str(pathlib.Path(p)) for the host: repeated and trailing separators and "."
// components dropped, ".." kept.
func Str(p string) string { return str(p, hostWindows) }

// Join is str(Path(a) / b) for the host, for a relative b.
func Join(a, b string) string { return join(a, b, hostWindows) }

// RelTo is str(path.relative_to(root)) for the host, for a path below root.
func RelTo(path, root string) string { return relTo(path, root, hostWindows) }

// Less is pathlib ordering for the host: by the list of components, case-folded on
// Windows.
func Less(a, b string) bool { return less(a, b, hostWindows) }

func str(p string, windows bool) string {
	if !windows {
		return normParts("", p, "/")
	}
	p = strings.ReplaceAll(p, "/", `\`)
	drive := winDrive(p)
	return normParts(drive, p[len(drive):], `\`)
}

// normParts is the drive followed by rest with empty and "." components dropped: rooted
// when rest starts with sep (always under a UNC share, as pathlib has it), "." when
// nothing is left.
func normParts(drive, rest, sep string) string {
	var parts []string
	for _, c := range strings.Split(rest, sep) {
		if c != "" && c != "." {
			parts = append(parts, c)
		}
	}
	s := strings.Join(parts, sep)
	switch {
	case strings.HasPrefix(rest, sep) || strings.HasPrefix(drive, `\\`):
		return drive + sep + s
	case drive != "":
		return drive + s
	case s == "":
		return "."
	}
	return s
}

// winDrive is the drive of a backslash-only Windows path: "X:" or a UNC "\\server\share".
func winDrive(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return p[:2]
	}
	if !strings.HasPrefix(p, `\\`) {
		return ""
	}
	parts := strings.SplitN(p[2:], `\`, 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return `\\` + parts[0] + `\` + parts[1]
}

func join(a, b string, windows bool) string {
	if windows {
		return str(pycompat.NtJoin(a, b), true)
	}
	if a == "." { // Path(".") / b is b
		return b
	}
	return pycompat.PosixJoin(a, b)
}

func relTo(path, root string, windows bool) string {
	if root == "." {
		return path
	}
	sep := "/"
	if windows {
		sep = `\`
	}
	return strings.TrimPrefix(path, strings.TrimSuffix(root, sep)+sep)
}

func less(a, b string, windows bool) bool {
	sep := "/"
	if windows {
		sep, a, b = `\`, strings.ToLower(a), strings.ToLower(b)
	}
	pa, pb := strings.Split(a, sep), strings.Split(b, sep)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}
