package pycompat

import (
	"os"
	"sort"
)

// Listdir is os.listdir(dir): the names in directory order (not sorted), nil and the open
// error when dir cannot be opened. Where dir opens but reading it fails (a file on Linux),
// the names read so far come back with the error, as (*os.File).Readdirnames returns them,
// so a caller that ignores the error sees what Readdirnames saw. The error is Go's as is:
// wrap it with WinListdirError where the Windows code must match Python's.
func Listdir(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// SortedListdir is sorted(os.listdir(dir)): Listdir with the names in byte order.
func SortedListdir(dir string) ([]string, error) {
	names, err := Listdir(dir)
	sort.Strings(names)
	return names, err
}
