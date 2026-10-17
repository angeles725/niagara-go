package pycompat

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// winErrorCodes are the codes compared with the Windows Python: common file and socket
// errors plus codes without a system message.
var winErrorCodes = []uint32{1, 2, 3, 5, 32, 87, 10013, 10048, 10049, 10050, 10051, 10054, 10060,
	10061, 10065, 11001, 11004, 0x2FFF, 0xFFFF, 0x7FFFFFFF, 0xFFFFFFFF}

// TestWinErrorStringMatchesPython runs the Windows Python named by PYCOMPAT_WINPY
// (testdata/winerror/pyerr.py) and compares its str(OSError) for every code, and for real
// refused-connection and address-in-use socket errors, with WinErrorString of the Win32
// code in the error Go returns. From WSL: WSLENV=PYCOMPAT_WINPY/p PYCOMPAT_WINPY=<python.exe>.
func TestWinErrorStringMatchesPython(t *testing.T) {
	py := os.Getenv("PYCOMPAT_WINPY")
	if py == "" {
		t.Skip("PYCOMPAT_WINPY is not set to a Windows python.exe; WinErrorString not compared with CPython")
	}
	out := filepath.Join(t.TempDir(), "pyerr.json")
	args := []string{"-I", filepath.Join("testdata", "winerror", "pyerr.py"), out}
	for _, c := range winErrorCodes {
		args = append(args, strconv.FormatUint(uint64(c), 10))
	}
	if b, err := exec.Command(py, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s pyerr.py: %v\n%s", py, err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Codes     map[string]string
		Refused   string
		AddrInUse string
	}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	for _, c := range winErrorCodes {
		k := strconv.FormatUint(uint64(c), 10)
		if got := WinErrorString(c); got != want.Codes[k] {
			t.Errorf("WinErrorString(%d) = %q, Python %q", c, got, want.Codes[k])
		}
	}

	_, err = net.DialTimeout("tcp", "127.0.0.1:1", 10*time.Second)
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
