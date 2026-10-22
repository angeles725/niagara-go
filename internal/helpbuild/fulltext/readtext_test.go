package fulltext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tokenize reads its file as open(path, encoding="utf-8", errors="replace").read():
// U+FFFD for undecodable bytes, universal newlines, and a skip when it cannot be read.
func TestTokenizeReadsReplaceText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("a\xff\r\nb\rc"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, ok := tokenize(file{"x", dir, path})
	if !ok || len(rows) != 1 || !strings.Contains(rows[0].doc.body, "a�\nb\nc") {
		t.Errorf("tokenize = %+v, %v; want one row whose body holds %q", rows, ok, "a�\nb\nc")
	}
	if _, ok := tokenize(file{"x", dir, filepath.Join(dir, "missing.txt")}); ok {
		t.Error("tokenize of a missing file succeeded")
	}
}
