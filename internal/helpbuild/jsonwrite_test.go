package helpbuild

import (
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

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
	if got := testkit.ReadText(t, path); got != `{"ok": 1}` {
		t.Fatalf("content %q, want {\"ok\": 1}", got)
	}
	testkit.NoDotEntries(t, filepath.Join(dir, "indexes"))
}
