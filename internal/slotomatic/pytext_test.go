package slotomatic

import (
	"os"
	"path/filepath"
	"testing"
)

// ReadSource is open(path, encoding="utf-8", errors="replace").read(): universal newlines
// and U+FFFD for undecodable bytes.
func TestReadSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "A.java")
	if err := os.WriteFile(path, []byte("class A {\r\n\tint x;\rint y;\n}\xff\r"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSource(path)
	if want := "class A {\n\tint x;\nint y;\n}�\n"; err != nil || got != want {
		t.Errorf("ReadSource = %q, %v; want %q", got, err, want)
	}
	if _, err := ReadSource(filepath.Join(t.TempDir(), "missing.java")); err == nil {
		t.Error("ReadSource of a missing file succeeded")
	}
}
