package pycompat

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func fsPath(t *testing.T, h *string) string {
	t.Helper()
	b, err := hex.DecodeString(*h)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOSErrorText(t *testing.T) {
	f := load(t)
	for _, c := range f.OSErrors {
		var n int
		var strerror, text, class string
		var name, name2 *string
		decode(t, c[0], &n)
		decode(t, c[1], &strerror)
		decode(t, c[2], &name)
		decode(t, c[3], &name2)
		decode(t, c[4], &text)
		decode(t, c[5], &class)
		var err error = syscall.Errno(n)
		switch {
		case name2 != nil:
			err = &os.LinkError{Op: "rename", Old: fsPath(t, name), New: fsPath(t, name2), Err: err}
		case name != nil:
			err = &fs.PathError{Op: "open", Path: fsPath(t, name), Err: err}
		default:
			err = os.NewSyscallError("read", err)
		}
		if got := osErrorText(err, linuxErrno()); got != text {
			t.Errorf("osErrorText(%v) = %q, want %q", err, got, text)
		}
		if got := osErrorName(err, linuxErrno()); got != class {
			t.Errorf("osErrorName(%v) = %q, want %q", err, got, class)
		}
		if got := linuxErrno().strerror(n); got != strerror {
			t.Errorf("strerror(%d) = %q, want %q", n, got, strerror)
		}
	}
	for _, c := range f.Strerror {
		var n int
		var want string
		decode(t, c[0], &n)
		decode(t, c[1], &want)
		if got := linuxErrno().strerror(n); got != want {
			t.Errorf("strerror(%d) = %q, want %q", n, got, want)
		}
	}
	if runtime.GOOS == "linux" {
		_, openErr := os.Open("/nonexistent-pycompat/x")
		renameErr := os.Rename("/nonexistent-pycompat/a", "/nonexistent-pycompat/b")
		for i, err := range []error{openErr, renameErr} {
			var want string
			decode(t, f.OSErrors[i][4], &want)
			if got := OSErrorText(err); got != want || OSErrorName(err) != "FileNotFoundError" {
				t.Errorf("OSErrorText(%v) = %q (%s), want %q", err, got, OSErrorName(err), want)
			}
			if got := Strerror(2); got != "No such file or directory" {
				t.Errorf("Strerror(2) = %q", got)
			}
		}
	}
}

func TestOSErrorTextWindows(t *testing.T) {
	samples := 0
	for _, line := range strings.Split(errnoWindows, "\n") {
		f := strings.SplitN(line, " ", 4)
		if f[0] != "X" {
			continue
		}
		samples++
		w, _ := strconv.Atoi(f[1])
		err := &fs.PathError{Op: "open", Path: "p", Err: syscall.Errno(w)}
		if got := osErrorText(err, windowsErrno()); got != f[3] {
			t.Errorf("Windows osErrorText(winerror %d) = %q, want %q", w, got, f[3])
		}
		if got := osErrorName(err, windowsErrno()); got != f[2] {
			t.Errorf("Windows osErrorName(winerror %d) = %q, want %q", w, got, f[2])
		}
	}
	if samples < 20 {
		t.Fatalf("only %d X samples in errno_windows.txt", samples)
	}
}

func TestStrerrorOf(t *testing.T) {
	f := load(t)
	checked := 0
	for _, c := range f.OSErrors {
		var n int
		var strerror string
		decode(t, c[0], &n)
		decode(t, c[1], &strerror)
		for _, err := range []error{
			syscall.Errno(n),
			&fs.PathError{Op: "open", Path: "p", Err: syscall.Errno(n)},
			&os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(n)},
			os.NewSyscallError("read", syscall.Errno(n)),
			fmt.Errorf("wrapped: %w", &fs.PathError{Op: "stat", Path: "q", Err: syscall.Errno(n)}),
		} {
			checked++
			if got := strerrorOf(err, linuxErrno()); got != strerror {
				t.Errorf("Linux strerrorOf(%#v) = %q, want %q", err, got, strerror)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no OSErrors fixtures")
	}
	// Windows: the Win32 code goes through winerror_to_errno first; the X samples hold
	// str(OSError) "[Errno n] <strerror>: 'p'" for each sampled winerror.
	for _, line := range strings.Split(errnoWindows, "\n") {
		x := strings.SplitN(line, " ", 4)
		if x[0] != "X" {
			continue
		}
		w, _ := strconv.Atoi(x[1])
		_, text, _ := strings.Cut(x[3], "] ")
		want := strings.TrimSuffix(text, ": 'p'")
		err := &fs.PathError{Op: "open", Path: "p", Err: syscall.Errno(w)}
		if got := strerrorOf(err, windowsErrno()); got != want {
			t.Errorf("Windows strerrorOf(winerror %d) = %q, want %q", w, got, want)
		}
	}
	for _, err := range []error{fs.ErrClosed, errors.New("plain"), &fs.PathError{Op: "open", Path: "p", Err: fs.ErrNotExist}} {
		if got := StrerrorOf(err); got != err.Error() {
			t.Errorf("StrerrorOf(%v) = %q, want err.Error()", err, got)
		}
	}
	if got := StrerrorOf(nil); got != "" {
		t.Errorf("StrerrorOf(nil) = %q, want empty", got)
	}
	if runtime.GOOS == "linux" {
		_, err := os.Open("/nonexistent-pycompat/x")
		if got := StrerrorOf(err); got != "No such file or directory" {
			t.Errorf("StrerrorOf(%v) = %q", err, got)
		}
	}
}

func TestOSErrorTextOfOtherErrors(t *testing.T) {
	err := fs.ErrClosed
	if got := OSErrorText(err); got != err.Error() {
		t.Errorf("OSErrorText(fs.ErrClosed) = %q", got)
	}
	if got := OSErrorName(err); got != "OSError" {
		t.Errorf("OSErrorName(fs.ErrClosed) = %q", got)
	}
}

func TestUniversalNewlines(t *testing.T) {
	for _, c := range load(t).Newlines {
		var s, text string
		var lines []string
		decode(t, c[0], &s)
		decode(t, c[1], &text)
		decode(t, c[2], &lines)
		if got := UniversalNewlines(s); got != text {
			t.Errorf("UniversalNewlines(%q) = %q, want %q", s, got, text)
		}
		if got := Lines(s); strings.Join(got, "|") != strings.Join(lines, "|") || len(got) != len(lines) {
			t.Errorf("Lines(%q) = %q, want %q", s, got, lines)
		}
	}
}

func TestFSRepr(t *testing.T) {
	for in, want := range map[string]string{"a": "'a'", "bad\xff\xfe": `'bad\udcff\udcfe'`, "é\xe2\x82": `'é\udce2\udc82'`,
		"it's\xff": `"it's\udcff"`, "\xed\xa0\x80": `'\udced\udca0\udc80'`} {
		if got := FSRepr(in); got != want {
			t.Errorf("FSRepr(%q) = %s, want %s", in, got, want)
		}
	}
}
