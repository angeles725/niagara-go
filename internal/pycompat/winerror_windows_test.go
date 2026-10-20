package pycompat

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// winErrorCodes are the codes frozen from the Windows Python: common file and socket errors
// plus codes without a system message.
var winErrorCodes = []uint32{1, 2, 3, 5, 32, 87, 10013, 10048, 10049, 10050, 10051, 10054, 10060,
	10061, 10065, 11001, 11004, 0x2FFF, 0xFFFF, 0x7FFFFFFF, 0xFFFFFFFF}

// readFrozen reads a testdata/winerror JSON file into v. The files hold what CPython 3.13
// (python.org build, Windows UI language 0x080A, es-MX) printed for the same failures; they
// were captured once with the retired pyerr.py/pyoserr.py scripts and are now the oracle.
// System messages follow the Windows UI language, so the test skips on any other language.
func readFrozen(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "winerror", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// frozenUILang is the Windows UI language the frozen texts were captured under.
const frozenUILang = 0x080A

func skipOtherUILanguage(t *testing.T) {
	t.Helper()
	lang, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	if uint16(lang) != frozenUILang {
		t.Skipf("Windows UI language %#04x; the frozen CPython texts are for %#04x", uint16(lang), frozenUILang)
	}
}

// TestWinErrorStringMatchesPython compares WinErrorString of every code, and of the Win32 code
// in real refused-connection and address-in-use socket errors Go returns, with the frozen
// str(OSError) CPython raised (PyErr_SetFromWindowsErr) in testdata/winerror/pyerr.json.
func TestWinErrorStringMatchesPython(t *testing.T) {
	skipOtherUILanguage(t)
	var want struct {
		UILang    int
		Codes     map[string]string
		Refused   string
		AddrInUse string
	}
	readFrozen(t, "pyerr.json", &want)
	if want.UILang != frozenUILang {
		t.Fatalf("pyerr.json uilang %#04x, want %#04x", want.UILang, frozenUILang)
	}
	for _, c := range winErrorCodes {
		k := strconv.FormatUint(uint64(c), 10)
		if got := WinErrorString(c); got != want.Codes[k] {
			t.Errorf("WinErrorString(%d) = %q, Python %q", c, got, want.Codes[k])
		}
	}

	_, err := net.DialTimeout("tcp", "127.0.0.1:1", 10*time.Second)
	compareSocketError(t, "dial 127.0.0.1:1", err, want.Refused)
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := net.Listen("tcp", first.Addr().String())
	if err == nil {
		second.Close()
	}
	compareSocketError(t, "listen twice", err, want.AddrInUse)
}

func compareSocketError(t *testing.T, what string, err error, python string) {
	t.Helper()
	var e syscall.Errno
	if !errors.As(err, &e) {
		t.Fatalf("%s: %v holds no syscall.Errno", what, err)
	}
	if got := WinErrorString(uint32(e)); got != python {
		t.Errorf("%s: WinErrorString(%d) = %q, Python %q", what, uint32(e), got, python)
	}
}
