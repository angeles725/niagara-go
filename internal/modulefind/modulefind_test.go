package modulefind

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are niagara-research tools/tests/fixtures/golden-cli/module-find.json (the Python
// output, converted by the retired testdata/convert_golden.py) plus Go-only --limit cases, run in testdata
// with the roots srcA, srcB and nojava relative to it.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenUnreadable runs module-find-unreadable.json like the Python suite: in a temporary
// directory holding src/, a copy of srcA whose com/acme/util/Helper.java is mode 000.
func TestGoldenUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a mode-000 file the test user cannot read")
	}
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "src"), os.DirFS(filepath.Join("testdata", "srcA"))); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(dir, "golden"), os.DirFS(filepath.Join("testdata", "unreadable", "golden"))); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "src", "com", "acme", "util", "Helper.java")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	golden.Run(t, dir, Main)
}

// TestMainWriteErrorExitsOne: output that cannot be written (a full disk) is named on stderr
// and exits 1, as the Python original's uncaught OSError, in every output mode.
func TestMainWriteErrorExitsOne(t *testing.T) {
	for _, mode := range [][]string{{"--csv"}, {"--json"}, nil} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			var errb bytes.Buffer
			out := &failAfter{n: 0}
			code := Main(append([]string{"testdata/srcA", "slots"}, mode...), cli.Stdio{Out: out, Err: &errb})
			want := "module-find: cannot write the output: " + errWrite.Error() + "\n"
			if code != 1 || errb.String() != want {
				t.Errorf("exit %d stderr %q, want exit 1 stderr %q", code, errb.String(), want)
			}
		})
	}
}

// TestMainKeepsTheFirstCSVWriteError: a CSV write that fails after earlier ones succeeded (a
// row, not the header) is the error Main reports, with exit 1, even though stdout would accept
// the next writes: the first error is kept and nothing more is written after it.
func TestMainKeepsTheFirstCSVWriteError(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			var errb bytes.Buffer
			out := &failOnNth{n: n}
			code := Main([]string{"testdata/srcA", "slots", "--csv"}, cli.Stdio{Out: out, Err: &errb})
			want := "module-find: cannot write the output: " + errWrite.Error() + "\n"
			if code != 1 || errb.String() != want || out.calls != n || out.bytesAfter != 0 {
				t.Errorf("exit %d stderr %q, %d writes, %d bytes after write %d; want exit 1 stderr %q, %d writes, 0 bytes",
					code, errb.String(), out.calls, out.bytesAfter, n, want, n)
			}
		})
	}
}

// TestNoReadableJavaIsNotNoJava: a root whose .java files all cannot be read is refused
// (exit 3) naming them, never reported as a root without .java files.
func TestNoReadableJavaIsNotNoJava(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a mode-000 file the test user cannot read")
	}
	root := filepath.Join(t.TempDir(), "src")
	locked := filepath.Join(root, "p", "BA.java")
	if err := os.MkdirAll(filepath.Dir(locked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locked, []byte("class BA {}\n"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	var out, errb bytes.Buffer
	code := Main([]string{root, "slots"}, cli.Stdio{Out: &out, Err: &errb})
	want := "module-find: unreadable, not searched: " + locked + ": Permission denied\n" +
		"module-find: no readable .java files under " + root + " (1 unreadable, dot-dirs pruned)\n"
	if code != 3 || out.String() != "" || errb.String() != want {
		t.Errorf("exit %d stdout %q stderr %q, want exit 3 stderr %q", code, out.String(), errb.String(), want)
	}
}
