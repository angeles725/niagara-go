package pycompat

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// The Windows rendering, with the off-Windows English texts of winerror_other.go; the real
// Windows messages are compared with CPython in TestWinOSErrorTextMatchesPython.
func TestWinOSErrorTextWindowsRendering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the English table is used only off Windows; see TestWinOSErrorTextMatchesPython")
	}
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connectex", syscall.Errno(10061))}
	for _, c := range []struct {
		err  error
		want string
	}{
		{refused, "[WinError 10061] No connection could be made because the target machine actively refused it"},
		{os.NewSyscallError("bind", syscall.Errno(10048)), "[WinError 10048] Only one usage of each socket address (protocol/network address/port) is normally permitted"},
		{syscall.Errno(5), "[WinError 5] Access is denied"},
		{&fs.PathError{Op: "stat", Path: `C:\x`, Err: syscall.Errno(3)}, `[WinError 3] The system cannot find the path specified: 'C:\\x'`},
		{fmt.Errorf("walk: %w", &fs.PathError{Op: "open", Path: `C:\it's`, Err: syscall.Errno(2)}), `[WinError 2] The system cannot find the file specified: "C:\\it's"`},
		{&os.LinkError{Op: "rename", Old: `a`, New: `b\c`, Err: syscall.Errno(2)}, `[WinError 2] The system cannot find the file specified: 'a' -> 'b\\c'`},
		{syscall.Errno(0x2FFF), "[WinError 12287] Windows Error 0x2fff"},
	} {
		if got := winOSErrorText(c.err, true); got != c.want {
			t.Errorf("winOSErrorText(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// Errors without a Win32 code, and every error off Windows, render as OSErrorText.
func TestWinOSErrorTextFallsBackToOSErrorText(t *testing.T) {
	invented := syscall.Errno(1<<29 + 7) // a code Go invents at APPLICATION_ERROR and above
	plain := errors.New("no errno here")
	missing := &fs.PathError{Op: "open", Path: "/x", Err: syscall.Errno(2)}
	for _, c := range []struct {
		err     error
		windows bool
	}{
		{plain, true}, {invented, true}, {plain, false}, {missing, false},
		{&net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.Errno(111))}, false},
	} {
		if got, want := winOSErrorText(c.err, c.windows), OSErrorText(c.err); got != want {
			t.Errorf("winOSErrorText(%v, %v) = %q, want OSErrorText %q", c.err, c.windows, got, want)
		}
	}
}

func TestWinOSErrorTextLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("errno numbers and strerror texts are Linux's")
	}
	for err, want := range map[error]string{
		&net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}: "[Errno 111] Connection refused",
		os.NewSyscallError("bind", syscall.EADDRINUSE):                                     "[Errno 98] Address already in use",
		&fs.PathError{Op: "stat", Path: "/no/such", Err: syscall.ENOENT}:                   "[Errno 2] No such file or directory: '/no/such'",
	} {
		if got := WinOSErrorText(err); got != want {
			t.Errorf("WinOSErrorText(%v) = %q, want %q", err, got, want)
		}
	}
}

// On Windows Go's open codes for a missing directory (2) and a file (3) become the codes
// FindFirstFileW gives CPython's os.listdir (3 and 267); other errors are kept.
func TestWinListdirError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pathErr := func(p string, code syscall.Errno) error { return &fs.PathError{Op: "open", Path: p, Err: code} }
	for _, c := range []struct {
		err  error
		want syscall.Errno
	}{
		{pathErr(filepath.Join(dir, "no"), 2), 3},
		{pathErr(filepath.Join(dir, "no", "such"), 3), 3},
		{pathErr(file, 3), 267},
		{pathErr(dir, 5), 5},
	} {
		got := winListdirError(c.err, true)
		var pe *fs.PathError
		if !errors.As(got, &pe) || pe.Err != c.want || pe.Path != c.err.(*fs.PathError).Path {
			t.Errorf("winListdirError(%v) = %v, want code %d", c.err, got, c.want)
		}
	}
	plain := errors.New("x")
	if got := winListdirError(plain, true); got != plain {
		t.Errorf("winListdirError(plain) = %v", got)
	}
	missing := pathErr(filepath.Join(dir, "no"), 2)
	if got := winListdirError(missing, false); got != missing {
		t.Errorf("off Windows winListdirError(%v) = %v, want it unchanged", missing, got)
	}
}

func TestWinStrerrorOf(t *testing.T) {
	missing := &fs.PathError{Op: "open", Path: "/x", Err: syscall.Errno(3)}
	if runtime.GOOS != "windows" {
		if got, want := winStrerrorOf(missing, true), "The system cannot find the path specified"; got != want {
			t.Errorf("winStrerrorOf(%v, true) = %q, want %q", missing, got, want)
		}
	}
	for _, err := range []error{missing, syscall.Errno(1<<29 + 7), errors.New("x"), nil} {
		if got, want := winStrerrorOf(err, false), StrerrorOf(err); got != want {
			t.Errorf("winStrerrorOf(%v, false) = %q, want StrerrorOf %q", err, got, want)
		}
	}
	if got, want := winStrerrorOf(syscall.Errno(1<<29+7), true), StrerrorOf(syscall.Errno(1<<29+7)); got != want {
		t.Errorf("winStrerrorOf(invented, true) = %q, want %q", got, want)
	}
}
