package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// modeOf is path's permission bits; the mode checks are skipped on Windows,
// which has no POSIX mode bits.
func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode().Perm()
}

func writeString(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

func TestWriteStreamWritesTheFileWithTheGivenMode(t *testing.T) {
	for _, perm := range []os.FileMode{0o600, 0o644} {
		path := filepath.Join(t.TempDir(), "a", "b", "cache.gob")
		if err := WriteStream(path, perm, writeString("data")); err != nil {
			t.Fatal(err)
		}
		if got := readText(t, path); got != "data" {
			t.Fatalf("content %q", got)
		}
		if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "cache.gob" {
			t.Fatalf("directory holds %q", got)
		}
		if runtime.GOOS != "windows" && modeOf(t, path) != perm {
			t.Fatalf("mode %o, want %o", modeOf(t, path), perm)
		}
	}
}

func TestWriteStreamReplacesAnExistingFile(t *testing.T) {
	path := atomicFixture(t)
	if err := WriteStream(path, 0o644, writeString("second")); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != "second" {
		t.Fatalf("content %q", got)
	}
	if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q", got)
	}
}

// A write that fails after a partial write keeps the previous file and leaves no temp.
func TestWriteStreamFailedWriteKeepsThePreviousFile(t *testing.T) {
	path := atomicFixture(t)
	failed := errors.New("encode failed")
	err := WriteStream(path, 0o644, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial")
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("err %v, want the write error", err)
	}
	if got := readText(t, path); got != `{"ok": 1}` {
		t.Fatalf("previous file changed: %q", got)
	}
	if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q, want only x.json", got)
	}
}

func TestWriteStreamFailedRenameLeavesNoTemp(t *testing.T) {
	path := atomicFixture(t)
	injected := failRename(t, 0)
	if err := WriteStream(path, 0o644, writeString("new")); !errors.Is(err, injected) {
		t.Fatalf("err %v, want the rename failure", err)
	}
	if got := readText(t, path); got != `{"ok": 1}` {
		t.Fatalf("previous file changed: %q", got)
	}
	if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q, want only x.json", got)
	}
}

func TestWriteStreamPanicRemovesTheTempAndRepanics(t *testing.T) {
	path := atomicFixture(t)
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recovered %v, want the write's panic", r)
		}
		if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
			t.Fatalf("directory holds %q, want only x.json", got)
		}
	}()
	_ = WriteStream(path, 0o644, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial")
		panic("boom")
	})
}

// TempPath reserves a distinct empty file beside final per call, with the mode a plain
// create (0666 less the umask) gives, and creates the missing parents.
func TestTempPathReservesAnEmptySibling(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "indexes")
	final := filepath.Join(dir, "fulltext.sqlite")
	a, err := TempPath(final)
	if err != nil {
		t.Fatal(err)
	}
	b, err := TempPath(final)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || filepath.Dir(a) != dir || filepath.Dir(b) != dir {
		t.Fatalf("temp paths %q, %q; want two distinct files in %s", a, b, dir)
	}
	if got := readText(t, a); got != "" {
		t.Fatalf("temp file holds %q", got)
	}
	if runtime.GOOS != "windows" {
		ref := filepath.Join(t.TempDir(), "ref")
		f, err := os.OpenFile(ref, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o666)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		if modeOf(t, a) != modeOf(t, ref) {
			t.Fatalf("temp mode %o, want %o (0666 less the umask)", modeOf(t, a), modeOf(t, ref))
		}
	}
}

func TestReplaceRenamesTheBuiltFileIn(t *testing.T) {
	final := atomicFixture(t)
	tmp := final + ".build"
	err := Replace(final, tmp, func(p string) error { return os.WriteFile(p, []byte("built"), 0o644) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readText(t, final); got != "built" {
		t.Fatalf("content %q", got)
	}
	if got := listDir(t, filepath.Dir(final)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q", got)
	}
}

// A failed build, a failed rename and a panic each discard the temp file (os.Remove
// when discard is nil, the given discard otherwise) and keep the previous file.
func TestReplaceFailuresDiscardTheTemp(t *testing.T) {
	built := func(p string) error { return os.WriteFile(p, []byte("built"), 0o644) }
	failed := errors.New("build failed")
	check := func(t *testing.T, final string) {
		t.Helper()
		if got := readText(t, final); got != `{"ok": 1}` {
			t.Fatalf("previous file changed: %q", got)
		}
		if got := listDir(t, filepath.Dir(final)); strings.Join(got, ",") != "x.json" {
			t.Fatalf("directory holds %q, want only x.json", got)
		}
	}
	t.Run("build", func(t *testing.T) {
		final := atomicFixture(t)
		err := Replace(final, final+".build", func(p string) error {
			_ = built(p)
			return failed
		}, nil)
		if !errors.Is(err, failed) {
			t.Fatalf("err %v, want the build error", err)
		}
		check(t, final)
	})
	t.Run("rename", func(t *testing.T) {
		final := atomicFixture(t)
		injected := failRename(t, 0)
		var discarded []string
		discard := func(p string) { discarded = append(discarded, p); _ = os.Remove(p) }
		if err := Replace(final, final+".build", built, discard); !errors.Is(err, injected) {
			t.Fatalf("err %v, want the rename failure", err)
		}
		if strings.Join(discarded, ",") != final+".build" {
			t.Fatalf("discarded %q, want the temp once", discarded)
		}
		check(t, final)
	})
	t.Run("panic", func(t *testing.T) {
		final := atomicFixture(t)
		defer func() {
			if r := recover(); r != "boom" {
				t.Fatalf("recovered %v, want the build's panic", r)
			}
			check(t, final)
		}()
		_ = Replace(final, final+".build", func(p string) error {
			_ = built(p)
			panic("boom")
		}, nil)
	})
}
