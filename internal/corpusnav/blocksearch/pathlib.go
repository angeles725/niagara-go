package blocksearch

import (
	"runtime"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The corpus tool builds every path with pathlib (config.py, corpus.py), so its paths are
// spelled the host's way: PurePosixPath off Windows; WindowsPath on Windows, where every
// "/" becomes "\" and docs/retros ids print as "docs\guide.md".

const hostWindows = runtime.GOOS == "windows"

// PyPath is str(pathlib.Path(p)): repeated and trailing separators and "." components
// dropped, ".." kept.
func PyPath(p string) string { return pyPath(p, hostWindows) }

// PyJoin is str(Path(a) / b) for a relative b.
func PyJoin(a, b string) string { return pyJoin(a, b, hostWindows) }

// RelTo is str(path.relative_to(root)) for a path below root.
func RelTo(path, root string) string { return relTo(path, root, hostWindows) }

// PathLess is pathlib ordering: by the list of components, case-folded on Windows.
func PathLess(a, b string) bool { return pathLess(a, b, hostWindows) }

func pyPath(p string, windows bool) string {
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

func pyJoin(a, b string, windows bool) string {
	if windows {
		return pyPath(pycompat.NtJoin(a, b), true)
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

func pathLess(a, b string, windows bool) bool {
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
