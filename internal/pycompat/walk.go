package pycompat

import (
	"io/fs"
	"os"
)

// WalkOptions are the os.walk arguments a caller can change: Unsorted lists each directory in
// the order the file system returns it (os.scandir's order) for the callers that do not sort
// what os.walk yields, and OnError is os.walk's onerror.
type WalkOptions struct {
	Unsorted bool
	OnError  func(dir string, err error)
}

// Walk is sorted os.walk(top) top-down without followlinks. visit receives each directory
// (joined like os.path.join, from top), the names of its subdirectories (a symlink to a
// directory counts as one, as DirEntry.is_dir() follows links) and of everything else, and
// returns the subdirectories to descend, like a caller editing dirnames in place. A symlinked
// subdirectory is listed but not descended. A directory that cannot be read is skipped
// silently (onerror=None).
func Walk(top string, visit func(dir string, dirs, files []string) []string) {
	WalkWith(top, WalkOptions{}, visit)
}

// WalkWith is Walk with the options o: an unreadable directory goes to o.OnError (as joined
// from top, with the error) when set, and the walk goes on without it.
func WalkWith(top string, o WalkOptions, visit func(dir string, dirs, files []string) []string) {
	list := os.ReadDir
	if o.Unsorted {
		list = readDirUnsorted
	}
	walk(top, list, o.OnError, visit)
}

// readDirUnsorted is os.ReadDir without the sort.
func readDirUnsorted(dir string) ([]fs.DirEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func walk(top string, list func(string) ([]fs.DirEntry, error), onerror func(dir string, err error),
	visit func(dir string, dirs, files []string) []string) {
	ents, err := list(top)
	if err != nil {
		if onerror != nil {
			onerror(top, err)
		}
		return
	}
	var dirs, files []string
	for _, e := range ents {
		isDirEntry := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			st, err := os.Stat(Join(top, e.Name()))
			isDirEntry = err == nil && st.IsDir()
		}
		if isDirEntry {
			dirs = append(dirs, e.Name())
		} else {
			files = append(files, e.Name())
		}
	}
	for _, d := range visit(top, dirs, files) {
		sub := Join(top, d)
		if st, err := os.Lstat(sub); err == nil && st.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		walk(sub, list, onerror, visit)
	}
}
