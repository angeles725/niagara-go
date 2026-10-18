package pycompat

import (
	"runtime"
	"strings"
)

// Join is os.path.join for the host: posixpath.join off Windows, ntpath.join on Windows.
// Nothing is normalised, so a "/" inside a part stays as given on Windows.
func Join(first string, rest ...string) string {
	if runtime.GOOS == "windows" {
		return NtJoin(first, rest...)
	}
	return PosixJoin(first, rest...)
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
