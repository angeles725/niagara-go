package scope

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// walkCount counts every file os.walk(dir) lists: symbolic links are not
// followed, a link to a directory is a directory (listed, not entered) and
// any other entry, a dangling link included, is a file. Unreadable
// directories are skipped silently, as os.walk does by default.
func walkCount(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, d := range entries {
		path := filepath.Join(dir, d.Name())
		switch {
		case d.IsDir():
			n += walkCount(path)
		case d.Type()&fs.ModeSymlink != 0 && isDir(path):
		default:
			n++
		}
	}
	return n
}

// isDir is os.path.isdir: true for a directory or a link to one.
func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// thousands is "{:,}".format(n) for n >= 0.
func thousands(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
