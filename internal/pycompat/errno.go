package pycompat

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// The errno tables were generated from Python: errno_linux.txt by the retired testdata/gen.py (glibc's
// strerror) and errno_windows.txt by the retired testdata/gen_windows.py run with a Windows Python
// (the C runtime's strerror and CPython's winerror_to_errno).
var (
	//go:embed errno_linux.txt
	errnoLinux string
	//go:embed errno_windows.txt
	errnoWindows string

	linuxErrno   = sync.OnceValue(func() *errnoTable { return parseErrno(errnoLinux, "Unknown error %d") })
	windowsErrno = sync.OnceValue(func() *errnoTable { return parseErrno(errnoWindows, "Unknown error") })
)

type winRun struct{ lo, hi, errno int } // errno -1: the code is its own errno

type errnoTable struct {
	text, class map[int]string
	runs        []winRun // Windows only: winerror -> errno, EINVAL (22) when absent
	unknown     string   // the strerror text of an unknown errno (may hold one %d)
}

func parseErrno(data, unknown string) *errnoTable {
	t := &errnoTable{text: map[int]string{}, class: map[int]string{}, unknown: unknown}
	for _, line := range strings.Split(data, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		kind, rest, _ := strings.Cut(line, " ")
		switch kind {
		case "E", "K":
			n, text, _ := strings.Cut(rest, " ")
			dst := t.text
			if kind == "K" {
				dst = t.class
			}
			dst[atoiOrPanic(n)] = text
		case "W":
			for _, e := range strings.Fields(rest) {
				span, to, _ := strings.Cut(e, ":")
				lo, hi, _ := strings.Cut(span, "-")
				r := winRun{atoiOrPanic(lo), atoiOrPanic(hi), -1}
				if to != "=" {
					r.errno = atoiOrPanic(to)
				}
				t.runs = append(t.runs, r)
			}
		case "X": // samples for the tests
		default:
			panic("pycompat: corrupt errno table line " + strconv.Quote(line))
		}
	}
	return t
}

func atoiOrPanic(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic("pycompat: corrupt errno table number " + strconv.Quote(s))
	}
	return n
}

func (t *errnoTable) strerror(n int) string {
	if s, ok := t.text[n]; ok {
		return s
	}
	if strings.Contains(t.unknown, "%d") {
		return fmt.Sprintf(t.unknown, n)
	}
	return t.unknown
}

// errnoOf returns the errno Python reports for err; on Windows (t.runs set) the Win32 code
// of a syscall.Errno goes through winerror_to_errno.
func (t *errnoTable) errnoOf(err error) (int, bool) {
	var e syscall.Errno
	if !errors.As(err, &e) {
		return 0, false
	}
	n := int(e)
	if t.runs == nil {
		return n, true
	}
	for _, r := range t.runs {
		if r.lo <= n && n <= r.hi {
			if r.errno < 0 {
				return n, true
			}
			return r.errno, true
		}
	}
	return 22, true // EINVAL, winerror_to_errno's default
}

func platformErrno() *errnoTable {
	if runtime.GOOS == "windows" {
		return windowsErrno()
	}
	return linuxErrno()
}

// StrerrorOf is e.strerror of the OSError Python raises where the Go os package returned
// err: os.strerror of the errno in err (glibc's text, or the C runtime's on Windows), found
// through *fs.PathError, *os.LinkError, *os.SyscallError and any other wrapping; on Windows
// the Win32 code first becomes an errno as in OSErrorText. An error that holds no syscall.Errno is returned as err.Error(), and
// nil as "".
func StrerrorOf(err error) string { return strerrorOf(err, platformErrno()) }

func strerrorOf(err error, t *errnoTable) string {
	if err == nil {
		return ""
	}
	n, ok := t.errnoOf(err)
	if !ok {
		return err.Error()
	}
	return t.strerror(n)
}

// OSErrorText is str() of the OSError Python raises where the Go os package returned err:
// "[Errno 2] No such file or directory: 'p'", "... : 'a' -> 'b'" for an *os.LinkError, and
// "[Errno 5] Input/output error" without a path. Names are shown as FSRepr. On Windows the
// Win32 code becomes an errno as the C runtime's open() reports it (Python's os.stat and
// friends say "[WinError n] <system message>" instead). An error that holds no
// syscall.Errno is returned as err.Error().
func OSErrorText(err error) string { return osErrorText(err, platformErrno()) }

// OSErrorName is the OSError subclass Python raises for err (FileNotFoundError,
// PermissionError, IsADirectoryError, ...), "OSError" when none applies.
func OSErrorName(err error) string { return osErrorName(err, platformErrno()) }

func osErrorText(err error, t *errnoTable) string {
	n, ok := t.errnoOf(err)
	if !ok {
		return err.Error()
	}
	head := fmt.Sprintf("[Errno %d] %s", n, t.strerror(n))
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

func osErrorName(err error, t *errnoTable) string {
	if n, ok := t.errnoOf(err); ok {
		if c, ok := t.class[n]; ok {
			return c
		}
	}
	return "OSError"
}
