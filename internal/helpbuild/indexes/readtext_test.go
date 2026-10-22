package indexes

import (
	"os"
	"path/filepath"
	"testing"
)

// readJavaText is open(path, encoding="utf-8", errors="replace").read(): U+FFFD for
// undecodable bytes, universal newlines, ok false when the file cannot be read.
func TestReadJavaText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "A.java")
	if err := os.WriteFile(path, []byte("class A {\xff\r\n}\r"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := readJavaText(path); !ok || got != "class A {�\n}\n" {
		t.Errorf("readJavaText = %q, %v; want %q, true", got, ok, "class A {�\n}\n")
	}
	for _, p := range []string{filepath.Join(dir, "missing.java"), dir} {
		if _, ok := readJavaText(p); ok {
			t.Errorf("readJavaText(%q) succeeded", p)
		}
	}
}
