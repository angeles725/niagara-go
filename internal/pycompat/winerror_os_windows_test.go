package pycompat

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestWinOSErrorTextMatchesPython runs the Windows Python named by PYCOMPAT_WINPY
// (testdata/winerror/pyoserr.py) and compares str(e) and type(e).__name__ of real socket and
// os failures with WinOSErrorText and OSErrorName of the error Go returns for the same call
// (through WinListdirError for os.listdir; OSErrorText for open(), which goes through the C
// runtime). From WSL: WSLENV=PYCOMPAT_WINPY/p PYCOMPAT_WINPY=<python.exe>.
func TestWinOSErrorTextMatchesPython(t *testing.T) {
	py := os.Getenv("PYCOMPAT_WINPY")
	if py == "" {
		t.Skip("PYCOMPAT_WINPY is not set to a Windows python.exe; WinOSErrorText not compared with CPython")
	}
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "pyoserr.json")
	script := filepath.Join("testdata", "winerror", "pyoserr.py")
	if b, err := exec.Command(py, "-I", script, out, dir).CombinedOutput(); err != nil {
		t.Fatalf("%s pyoserr.py: %v\n%s", py, err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]struct{ Text, Class string }
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	p := func(parts ...string) string { return filepath.Join(append([]string{dir}, parts...)...) }
	if err := os.WriteFile(p("file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, refused := net.DialTimeout("tcp", "127.0.0.1:1", 10*time.Second)
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, addrInUse := net.Listen("tcp", first.Addr().String())
	if addrInUse == nil {
		second.Close()
	}
	_, stat := os.Stat(p("no", "such"))
	_, statFile := os.Stat(p("missing.txt"))
	_, listDir := os.ReadDir(p("no"))
	listDir = WinListdirError(listDir)
	_, listFile := os.ReadDir(p("file.txt"))
	listFile = WinListdirError(listFile)
	mkdir := os.Mkdir(p("no", "sub"), 0o777)
	rename := os.Rename(p("missing.txt"), p("dst.txt"))
	_, open := os.ReadFile(p("missing.txt"))

	for name, err := range map[string]error{"refused": refused, "addrinuse": addrInUse, "stat": stat,
		"statfile": statFile, "listdir": listDir, "listfile": listFile, "mkdir": mkdir, "rename": rename} {
		compareOSError(t, name, err, WinOSErrorText(err), want[name])
	}
	compareOSError(t, "open", open, OSErrorText(open), want["open"])
}

func compareOSError(t *testing.T, name string, err error, got string, want struct{ Text, Class string }) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: Go returned no error, Python %q", name, want.Text)
		return
	}
	if got != want.Text {
		t.Errorf("%s: Go %q (%v), Python %q", name, got, err, want.Text)
	}
	if c := OSErrorName(err); c != want.Class {
		t.Errorf("%s: OSErrorName %q, Python %s", name, c, want.Class)
	}
}
