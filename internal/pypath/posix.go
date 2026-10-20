package pypath

import (
	"path"
	"strings"
)

// The posixpath and ntpath string functions (moved from pycompat): nothing is normalised
// except by PosixNormpath, unlike the pathlib forms in pypath.go.

// PosixBasename is posixpath.basename.
func PosixBasename(p string) string { return p[strings.LastIndex(p, "/")+1:] }

// PosixSplitext is posixpath.splitext: (root, ext), leading dots of the name are not an ext.
func PosixSplitext(p string) (string, string) {
	sep, dot := strings.LastIndex(p, "/"), strings.LastIndex(p, ".")
	if dot > sep {
		for i := sep + 1; i < dot; i++ {
			if p[i] != '.' {
				return p[:dot], p[dot:]
			}
		}
	}
	return p, ""
}

// PosixNormpath is posixpath.normpath on every OS: path.Clean, except that only "/"
// separates (a backslash or a drive letter is part of a name) and two leading slashes are
// kept (three or more become one).
func PosixNormpath(p string) string {
	clean := path.Clean(p)
	if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		return "/" + clean
	}
	return clean
}

// PosixJoin is posixpath.join: an absolute part restarts the path; nothing is normalised.
func PosixJoin(first string, rest ...string) string {
	out := first
	for _, p := range rest {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// NtJoin is CPython's ntpath.join: a rooted part restarts the path (keeping the drive), a
// part on another drive replaces it, and a backslash goes between parts unless one already
// ends in a separator. Drive letters compare case-insensitively; the later spelling wins.
func NtJoin(first string, rest ...string) string {
	drive, root, path := ntSplitRoot(first)
	for _, p := range rest {
		pDrive, pRoot, pPath := ntSplitRoot(p)
		if pRoot != "" {
			if pDrive != "" || drive == "" {
				drive = pDrive
			}
			root, path = pRoot, pPath
			continue
		}
		if pDrive != "" && pDrive != drive {
			if !strings.EqualFold(pDrive, drive) {
				drive, root, path = pDrive, pRoot, pPath
				continue
			}
			drive = pDrive
		}
		if path != "" && !strings.ContainsRune(`\/`, rune(path[len(path)-1])) {
			path += `\`
		}
		path += pPath
	}
	if path != "" && root == "" && drive != "" && !strings.ContainsRune(`:\/`, rune(drive[len(drive)-1])) {
		return drive + `\` + path
	}
	return drive + root + path
}

// ntSplitRoot is ntpath.splitroot: (drive, root, tail) of p, where the drive is "X:", a UNC
// "\\server\share" (also "\\?\UNC\server\share") or a device "\\.\name", either separator.
func ntSplitRoot(p string) (drive, root, tail string) {
	norm := strings.ReplaceAll(p, "/", `\`)
	switch {
	case strings.HasPrefix(norm, `\\`):
		start := 2
		if len(norm) >= 8 && strings.EqualFold(norm[:8], `\\?\UNC\`) {
			start = 8
		}
		i := strings.IndexByte(norm[start:], '\\')
		if i < 0 {
			return p, "", ""
		}
		i += start
		j := strings.IndexByte(norm[i+1:], '\\')
		if j < 0 {
			return p, "", ""
		}
		j += i + 1
		return p[:j], p[j : j+1], p[j+1:]
	case strings.HasPrefix(norm, `\`):
		return "", p[:1], p[1:]
	case len(norm) >= 2 && norm[1] == ':':
		if len(norm) >= 3 && norm[2] == '\\' {
			return p[:2], p[2:3], p[3:]
		}
		return p[:2], "", p[2:]
	}
	return "", "", p
}
