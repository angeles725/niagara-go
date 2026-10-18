package helpbuild

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func noDotEntries(t *testing.T, root string) {
	t.Helper()
	for _, n := range listDir(t, root) {
		if strings.HasPrefix(n, ".") {
			t.Fatalf("stray temp entry %s left in %s", n, root)
		}
	}
}

// test_atomic_writes.TestAtomicWrite.setUp: write_json(path, {"ok": 1}); the
// write itself (temp file, rename, mode) is tested in internal/atomicfile.
func TestWriteJSONCreatesParentsWithPythonDefaultDump(t *testing.T) {
	v := pyjson.NewObject()
	v.Set("ok", pyjson.IntValue(1))
	dir := t.TempDir()
	path := filepath.Join(dir, "indexes", "x.json")
	if err := WriteJSON(path, v, DumpDefault); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != `{"ok": 1}` {
		t.Fatalf("content %q, want {\"ok\": 1}", got)
	}
	noDotEntries(t, filepath.Join(dir, "indexes"))
}
