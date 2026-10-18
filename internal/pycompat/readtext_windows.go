package pycompat

import (
	"io/fs"
	"os"
	"syscall"
)

// errorAccessDenied is the Win32 ERROR_ACCESS_DENIED, which the C runtime maps to EACCES.
const errorAccessDenied = 5

// readFile is Python's open(path).read() on Windows: the C runtime's open() refuses a
// directory with ERROR_ACCESS_DENIED ("[Errno 13] Permission denied"), where Go opens it
// and only the read fails (ERROR_INVALID_FUNCTION, "Invalid argument"). Same approach as
// hdbread.readFile.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil && IsDir(path) {
		return nil, &fs.PathError{Op: "open", Path: path, Err: syscall.Errno(errorAccessDenied)}
	}
	return data, err
}
