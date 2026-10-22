package dashpreview

import (
	"os"
	"path/filepath"
	"testing"
)

// findReaderJava is the first *Reader.java of os.walk over rcDir's parent: a directory's files
// before its subdirectories, names sorted. A symlink to a directory is one of os.walk's
// dirnames, never a match, and it is not entered.
func TestFindReaderJavaWalksLikeOsWalk(t *testing.T) {
	top := t.TempDir()
	for _, rel := range []string{"rc/x.json", "a/AReader.java", "b/BReader.java", "out/0Reader.java"} {
		p := filepath.Join(top, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range []string{"LReader.java", "0link"} {
		if err := os.Symlink(filepath.Join(top, "out"), filepath.Join(top, link)); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	rc := filepath.Join(top, "rc")
	if got, want := findReaderJava(rc), filepath.Join(top, "a", "AReader.java"); got != want {
		t.Errorf("findReaderJava = %q, want %q", got, want)
	}
	if err := os.WriteFile(filepath.Join(top, "TopReader.java"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := findReaderJava(rc), filepath.Join(top, "TopReader.java"); got != want {
		t.Errorf("findReaderJava = %q, want %q (files before subdirectories)", got, want)
	}
	if got := findReaderJava(filepath.Join(top, "absent", "rc")); got != "" {
		t.Errorf("findReaderJava under a missing parent = %q", got)
	}
}
