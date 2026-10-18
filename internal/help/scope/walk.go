package scope

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/pycompat"
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
		case d.Type()&fs.ModeSymlink != 0 && pycompat.IsDir(path):
		default:
			n++
		}
	}
	return n
}
