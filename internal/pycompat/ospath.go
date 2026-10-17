package pycompat

import "os"

// IsDir is os.path.isdir(p): true for a directory or a symlink to one; any stat error
// (missing, unreadable, a NUL in p) is false.
func IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// IsFile is os.path.isfile(p): true for a regular file or a symlink to one; any stat error is
// false.
func IsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
