package pycompat

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
	"syscall"
)

// goInventedErrno is syscall.APPLICATION_ERROR on Windows: Go invents the Errno values at and
// above it (syscall.EINVAL, ENOTSUP, ...), so they are not Win32 codes Python could report.
const goInventedErrno = 1 << 29

// WinOSErrorText is str() of the OSError CPython raises where a socket call or an os function
// backed by a Win32 API (os.stat, os.listdir, os.scandir, os.mkdir, os.rename, ...) failed and
// Go returned err. On Windows that is "[WinError n] <WinErrorText(n)>" for the Win32 or
// Winsock code in err, followed by ": 'p'" for an *fs.PathError and ": 'a' -> 'b'" for an
// *os.LinkError (names as FSRepr); socket errors (*net.OpError, *os.SyscallError) carry no
// name. Where Python goes through the C runtime instead (open(), os.open) the text is
// OSErrorText. Off Windows, and for an err without a Win32 code (no syscall.Errno, or one Go
// invented), it is OSErrorText(err): Python on Linux prints "[Errno n] <strerror>".
//
// The exception class is OSErrorName(err): on Windows it already maps the code through
// winerror_to_errno, as CPython picks the OSError subclass (10061 is ConnectionRefusedError).
func WinOSErrorText(err error) string { return winOSErrorText(err, runtime.GOOS == "windows") }

func winOSErrorText(err error, windows bool) string {
	var e syscall.Errno
	if !windows || !errors.As(err, &e) || uint64(e) >= goInventedErrno {
		return OSErrorText(err)
	}
	head := WinErrorString(uint32(e))
	var le *os.LinkError
	var pe *fs.PathError
	switch {
	case errors.As(err, &le):
		return head + ": " + FSRepr(le.Old) + " -> " + FSRepr(le.New)
	case errors.As(err, &pe):
		return head + ": " + FSRepr(pe.Path)
	}
	return head
}

// WinStrerrorOf is e.strerror of that OSError: on Windows WinErrorText of the Win32 code in
// err (what os.walk's onerror or a socket error's strerror holds), otherwise StrerrorOf(err).
func WinStrerrorOf(err error) string { return winStrerrorOf(err, runtime.GOOS == "windows") }

func winStrerrorOf(err error, windows bool) string {
	var e syscall.Errno
	if !windows || !errors.As(err, &e) || uint64(e) >= goInventedErrno {
		return StrerrorOf(err)
	}
	return WinErrorText(uint32(e))
}

// Win32 codes FindFirstFileW reports for "<dir>\*.*" (CPython's os.listdir and os.scandir).
const (
	errorFileNotFound = 2   // ERROR_FILE_NOT_FOUND: what Go's open says for a missing directory
	errorPathNotFound = 3   // ERROR_PATH_NOT_FOUND: the directory does not exist
	errorDirectory    = 267 // ERROR_DIRECTORY: the name is not a directory
)

// WinListdirError is the error CPython's os.listdir or os.scandir (and so os.walk's onerror)
// reports where os.ReadDir returned err: on Windows Python asks FindFirstFileW for
// "<path>\*.*", which says ERROR_PATH_NOT_FOUND (3) for a missing directory and
// ERROR_DIRECTORY (267) for a file, where Go's open says ERROR_FILE_NOT_FOUND (2) and
// ERROR_PATH_NOT_FOUND (3); the *fs.PathError gets that code, so WinOSErrorText and
// OSErrorName render it as Python does. Any other error, and every error off Windows (Go and
// Python both report open's errno there), is returned as is.
func WinListdirError(err error) error { return winListdirError(err, runtime.GOOS == "windows") }

func winListdirError(err error, windows bool) error {
	var pe *fs.PathError
	var e syscall.Errno
	if !windows || !errors.As(err, &pe) || !errors.As(pe.Err, &e) {
		return err
	}
	code := e
	switch e {
	case errorFileNotFound:
		code = errorPathNotFound
	case errorPathNotFound:
		if fi, serr := os.Stat(pe.Path); serr == nil && !fi.IsDir() {
			code = errorDirectory
		}
	}
	if code == e {
		return err
	}
	return &fs.PathError{Op: pe.Op, Path: pe.Path, Err: code}
}
