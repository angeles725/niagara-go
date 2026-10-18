package navbuild

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

func TestWriteIndexStream(t *testing.T) {
	base := newBase(t)
	path := IndexPath(base, MethodFile)
	mustWrite(t, path, "old")
	failed := errors.New("parse failed")
	err := writeIndexStream(base, MethodFile, func(w *bufio.Writer) error {
		w.WriteString("partial")
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("err = %v, want the fill error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("a failed write changed the index: %q", got)
	}
	if tmps, _ := filepath.Glob(filepath.Join(base, "indexes", ".tmp-*")); len(tmps) != 0 {
		t.Errorf("temp files left: %v", tmps)
	}
	if err := writeIndexStream(base, MethodFile, func(w *bufio.Writer) error {
		_, err := w.WriteString("new")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" || runtime.GOOS != "windows" && st.Mode().Perm() != 0o644 {
		t.Errorf("index %q, mode %v; want \"new\" with mode 0644", got, st.Mode().Perm())
	}
}

// TestTokenIndexInOddBaseDir builds the token database under a directory
// whose name holds the characters a SQLite URI would otherwise treat as
// query, fragment or separator.
func TestTokenIndexInOddBaseDir(t *testing.T) {
	odd := "a?b#c d%20"
	if runtime.GOOS == "windows" {
		odd = "a#c d%20" // "?" is not allowed in a Windows file name
	}
	base := filepath.Join(t.TempDir(), odd)
	organized := t.TempDir()
	mustWrite(t, filepath.Join(organized, "m", "A.java"), "class Alpha {}\n")
	mustWrite(t, IndexPath(base, ClassIndexFile), `{"classes": {"Alpha": [{"path": "m/A.java", "module": "m"}]}}`)
	ti, err := BuildTokenIndex(base, organized, DefaultMinTokenLen)
	if err != nil {
		t.Fatal(err)
	}
	if ti.FilesScanned != 1 || ti.TotalPostings != 1 || !pycompat.IsFile(IndexPath(base, TokenFile)) {
		t.Errorf("index %+v", ti)
	}
	if entries, _ := os.ReadDir(filepath.Dir(base)); len(entries) != 1 {
		t.Errorf("the database landed outside the base dir: %d entries", len(entries))
	}
}
