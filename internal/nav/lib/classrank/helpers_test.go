package classrank

import (
	"os"
	"path/filepath"
	"testing"
)

// skipsIndex is the rank-skips golden fixture: a mirror copy, an inner class,
// a null module and two modules of one name.
func skipsIndex(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixture", "rank-skips", "indexes", ClassIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newBase writes indexes/<name> files under a temporary base directory.
func newBase(t *testing.T, files map[string][]byte) string {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(base, "indexes", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}
