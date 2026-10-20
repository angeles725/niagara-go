package pycompat

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The frozen texts carry the es-MX system messages, so the byte-for-byte tests skip on any
// other Windows UI language. These tests run on every UI language: they keep what does not
// depend on it (the "[WinError N] " / "[Errno N] " prefix, the ": 'path'" suffix, the
// exception class, CPython's fixed "Windows Error 0x..." text for codes without a system
// message) exact, and require only a non-empty system message in between.

// splitErrorPrefix splits "[WinError 2] msg" (or "[Errno 2] msg") after its "] ".
func splitErrorPrefix(t *testing.T, s string) (prefix, rest string) {
	t.Helper()
	i := strings.Index(s, "] ")
	if !strings.HasPrefix(s, "[") || i < 0 {
		t.Fatalf("%q has no [WinError N] / [Errno N] prefix", s)
	}
	return s[:i+2], s[i+2:]
}

// checkWinErrorShape requires got to have want's prefix and a non-empty message, or to equal
// want exactly when want is CPython's language-independent text for a code with no message.
func checkWinErrorShape(t *testing.T, what, got, want string) {
	t.Helper()
	wantPrefix, wantMsg := splitErrorPrefix(t, want)
	if strings.HasPrefix(wantMsg, "Windows Error 0x") {
		if got != want {
			t.Errorf("%s = %q, Python %q", what, got, want)
		}
		return
	}
	if gotPrefix, gotMsg := splitErrorPrefix(t, got); gotPrefix != wantPrefix || gotMsg == "" {
		t.Errorf("%s = %q, want prefix %q and a non-empty message", what, got, wantPrefix)
	}
}

func TestWinErrorShapeAnyUILanguage(t *testing.T) {
	var want struct {
		Codes     map[string]string
		Refused   string
		AddrInUse string
	}
	readFrozen(t, "pyerr.json", &want)
	for _, c := range winErrorCodes {
		checkWinErrorShape(t, "WinErrorString("+strconv.FormatUint(uint64(c), 10)+")",
			WinErrorString(c), want.Codes[strconv.FormatUint(uint64(c), 10)])
	}

	socket := func(what string, err error, python string) {
		t.Helper()
		var e syscall.Errno
		if !errors.As(err, &e) {
			t.Fatalf("%s: %v holds no syscall.Errno", what, err)
		}
		checkWinErrorShape(t, what, WinErrorString(uint32(e)), python)
	}
	_, err := net.DialTimeout("tcp", "127.0.0.1:1", 10*time.Second)
	socket("dial 127.0.0.1:1", err, want.Refused)
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := net.Listen("tcp", first.Addr().String())
	if err == nil {
		second.Close()
	}
	socket("listen twice", err, want.AddrInUse)
}

func TestWinOSErrorShapeAnyUILanguage(t *testing.T) {
	var want map[string]frozenOSError
	readFrozen(t, "pyoserr.json", &want)
	errs, open := realOSErrors(t, want)
	errs["open"] = open
	for name, err := range errs {
		w := want[name]
		if err == nil {
			t.Errorf("%s: Go returned no error, Python %q", name, w.Text)
			continue
		}
		prefix, rest := splitErrorPrefix(t, w.Text)
		suffix, ok := strings.CutPrefix(rest, w.Strerror)
		if !ok || w.Strerror == "" {
			t.Fatalf("%s: frozen text %q does not hold its strerror %q", name, w.Text, w.Strerror)
		}
		text, strerror := WinOSErrorText(err), WinStrerrorOf(err)
		if name == "open" {
			text, strerror = OSErrorText(err), StrerrorOf(err)
		}
		if strerror == "" || text != prefix+strerror+suffix {
			t.Errorf("%s: Go %q (strerror %q), want %q + strerror + %q", name, text, strerror, prefix, suffix)
		}
		if c := OSErrorName(err); c != w.Class {
			t.Errorf("%s: OSErrorName %q, Python %s", name, c, w.Class)
		}
	}
}
