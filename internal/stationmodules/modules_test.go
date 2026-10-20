package stationmodules

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden replays the cases frozen from tools/station-modules.py by the retired testdata/capture.py.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestFixCopiesTheStagedJar replays testdata/copy (frozen from Python on a scratch copy of
// fixtures/install-copy): --fix copies the jar verbatim, with its modification time.
func TestFixCopiesTheStagedJar(t *testing.T) {
	td, _ := filepath.Abs("testdata")
	tmp := t.TempDir()
	copyTree(t, filepath.Join(td, "fixtures", "install-copy"), filepath.Join(tmp, "install-copy"))
	if err := os.WriteFile(filepath.Join(tmp, "kit.xml"), []byte(readFile(t, filepath.Join(td, "fixtures", "kit.xml"))), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(tmp, "install-copy", "sw", "4.14.0.162", "kitPid-rt.jar")
	mtime := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(src, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)
	code, out, errText := run("check", "kit.xml", "--install", "install-copy", "--fix")
	want, _ := strconv.Atoi(strings.TrimSpace(readFile(t, filepath.Join(td, "copy", "exit"))))
	// The staged path is os.path.join output (native separator); the recording is Linux's.
	wantOut := strings.ReplaceAll(readFile(t, filepath.Join(td, "copy", "stdout")),
		"install-copy/sw/4.14.0.162/kitPid-rt.jar", filepath.Join("install-copy", "sw", "4.14.0.162", "kitPid-rt.jar"))
	if code != want || out != wantOut || errText != readFile(t, filepath.Join(td, "copy", "stderr")) {
		t.Fatalf("exit %d\n%s%s", code, out, errText)
	}
	dst := filepath.Join(tmp, "install-copy", "modules", "kitPid-rt.jar")
	if readFile(t, dst) != readFile(t, src) {
		t.Error("copied jar differs from the staged one")
	}
	if st, err := os.Stat(dst); err != nil || !st.ModTime().Equal(mtime) {
		t.Errorf("copy2 keeps the modification time: %v %v", st.ModTime(), err)
	}
	if code, out, _ = run("check", "kit.xml", "--install", "install-copy", "--fix"); code != 0 || !strings.Contains(out, "OK — all required") {
		t.Errorf("second run after the copy: exit %d\n%s", code, out)
	}
}

// Ported from tools/tests/test_station_modules.py.

func TestAutodetectUsesTheFirstCandidateWithModules(t *testing.T) {
	td, _ := filepath.Abs("testdata/fixtures")
	old := installCandidates
	defer func() { installCandidates = old }()
	installCandidates = []string{filepath.Join(td, "nope"), filepath.Join(td, "no-modules"), filepath.Join(td, "install-ok")}
	code, out, _ := run("doctor")
	if code != 0 || !strings.HasPrefix(out, "install : "+filepath.Join(td, "install-ok")+"\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
	installCandidates = installCandidates[:2]
	if code, _, errText := run("doctor"); code != 1 || errText != "error: could not autodetect a Niagara install; pass --install ROOT\n" {
		t.Errorf("exit %d %q", code, errText)
	}
}
