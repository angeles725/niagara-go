package core

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type gobRecord struct {
	Name string
	N    int
}

// TestWriteGob: the file holds the gob of the value (pointer or not), in a
// directory created on demand (mode 0700), with mode 0600, and no temporary is left.
func TestWriteGob(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mnav-cache")
	path := filepath.Join(dir, "x.gob")
	for _, v := range []any{gobRecord{"a", 1}, &gobRecord{"a", 1}} {
		if !WriteGob(path, v) {
			t.Fatalf("WriteGob(%#v) = false", v)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var got gobRecord
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&got); err != nil || got != (gobRecord{"a", 1}) {
			t.Errorf("read back %+v, %v", got, err)
		}
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("mode %v, %v; want 0600", st.Mode().Perm(), err)
		}
		if st, err := os.Stat(dir); err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("directory mode %v, %v; want 0700", st.Mode().Perm(), err)
		}
	}
	assertOnly(t, dir, "x.gob")
}

// TestWriteGobFailures: an unencodable value, an unwritable directory and a
// rename onto a directory all return false and leave no temporary behind.
func TestWriteGobFailures(t *testing.T) {
	root := t.TempDir()
	if WriteGob(filepath.Join(root, "f.gob"), func() {}) {
		t.Error("unencodable value written")
	}
	assertOnly(t, root)
	blocker := filepath.Join(root, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if WriteGob(filepath.Join(blocker, "x.gob"), gobRecord{}) {
		t.Error("written under a regular file")
	}
	target := filepath.Join(root, "dir.gob")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if WriteGob(target, gobRecord{}) {
		t.Error("renamed onto a non-empty directory")
	}
	assertOnly(t, root, "dir.gob", "file")
}

func assertOnly(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if len(got) != len(names) {
		t.Fatalf("%s holds %q, want %q", dir, got, names)
	}
	for i := range names {
		if got[i] != names[i] {
			t.Fatalf("%s holds %q, want %q", dir, got, names)
		}
	}
}

// TestResolvedPath: a link resolves to its target; a missing path stays the
// absolute path (its parents are not resolved, unlike realpath).
func TestResolvedPath(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if got := ResolvedPath(link); got != real {
		t.Errorf("ResolvedPath(link) = %q, want %q", got, real)
	}
	missing := filepath.Join(link, "missing")
	if got := ResolvedPath(missing); got != missing {
		t.Errorf("ResolvedPath(missing) = %q, want %q", got, missing)
	}
	wd, _ := os.Getwd()
	if got := ResolvedPath("no-such-file"); got != filepath.Join(wd, "no-such-file") {
		t.Errorf("ResolvedPath(relative) = %q", got)
	}
}
