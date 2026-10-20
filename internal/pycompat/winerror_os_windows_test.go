package pycompat

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWinOSErrorTextMatchesPython compares WinOSErrorText, WinStrerrorOf and OSErrorName of
// the errors Go returns for real socket and os failures with the frozen str(e), e.strerror and
// type(e).__name__ CPython gave for the same calls in testdata/winerror/pyoserr.json (through
// WinListdirError for os.listdir; OSErrorText and StrerrorOf for open(), which goes through
// the C runtime). "{dir}" in the frozen texts stands for the repr of the test directory.
func TestWinOSErrorTextMatchesPython(t *testing.T) {
	skipOtherUILanguage(t)
	var want map[string]struct{ Text, Strerror, Class string }
	readFrozen(t, "pyoserr.json", &want)
	dir := t.TempDir()
	if strings.ContainsAny(dir, `'"`) {
		t.Skipf("temp dir %q needs a quoted repr the frozen texts do not cover", dir)
	}
	reprDir := strings.ReplaceAll(dir, `\`, `\\`)
	for k, w := range want {
		w.Text = strings.ReplaceAll(w.Text, "{dir}", reprDir)
		want[k] = w
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

func compareOSError(t *testing.T, name string, err error, got string, want struct{ Text, Strerror, Class string }) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: Go returned no error, Python %q", name, want.Text)
		return
	}
	if got != want.Text {
		t.Errorf("%s: Go %q (%v), Python %q", name, got, err, want.Text)
	}
	strerror := WinStrerrorOf(err)
	if name == "open" {
		strerror = StrerrorOf(err)
	}
	if strerror != want.Strerror {
		t.Errorf("%s: strerror %q, Python %q", name, strerror, want.Strerror)
	}
	if c := OSErrorName(err); c != want.Class {
		t.Errorf("%s: OSErrorName %q, Python %s", name, c, want.Class)
	}
}
