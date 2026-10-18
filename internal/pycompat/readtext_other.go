//go:build !windows

package pycompat

import "os"

// readFile is os.ReadFile: a directory already fails with EISDIR, as Python's open() does.
func readFile(path string) ([]byte, error) { return os.ReadFile(path) }
