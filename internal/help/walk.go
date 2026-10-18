package help

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// pyWalkFiles calls fn for every file os.walk(dir) lists: it does not follow
// symbolic links, a link to a directory is a directory (listed, not entered)
// and any other entry, a dangling link included, is a file. Unreadable
// directories are skipped silently, as os.walk does by default.
func pyWalkFiles(dir string, fn func(path string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range entries {
		path := filepath.Join(dir, d.Name())
		switch {
		case d.IsDir():
			pyWalkFiles(path, fn)
		case d.Type()&fs.ModeSymlink != 0 && pycompat.IsDir(path):
			// os.walk lists it under dirs and does not descend (followlinks=False)
		default:
			fn(path)
		}
	}
}
