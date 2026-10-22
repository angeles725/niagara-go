package inventory

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// _java_files walks each vineflower/ tree (and docSource's extracted/ one, keyed by its
// vineflower copy) like os.walk: modules and subfolders sorted, a folder's files before its
// subfolders, a symlinked folder not entered and non-.java files left out.
func TestJavaFilesWalkOrder(t *testing.T) {
	org := t.TempDir()
	for _, p := range []string{
		"m/s/vineflower/p/A.java", "m/s/vineflower/p/q/B.java", "m/s/vineflower/note.txt",
		"docSource/docSource-doc/extracted/x/C.java", "docSource/docSource-doc/vineflower/x/C.java",
	} {
		full := filepath.Join(org, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(org, "m/s/vineflower/p/q"), filepath.Join(org, "m/s/vineflower/p/link")); err != nil {
		t.Skipf("no symlinks here: %v", err) // Windows without the privilege
	}
	want := []recallFile{
		{"docSource/docSource-doc/vineflower/x/C.java", "docSource/docSource-doc/vineflower/x/C.java"},
		{"docSource/docSource-doc/extracted/x/C.java", "docSource/docSource-doc/vineflower/x/C.java"},
		{"m/s/vineflower/p/A.java", "m/s/vineflower/p/A.java"},
		{"m/s/vineflower/p/q/B.java", "m/s/vineflower/p/q/B.java"},
	}
	if got := javaFiles(org); !reflect.DeepEqual(got, want) {
		t.Errorf("javaFiles:\n got %q\nwant %q", got, want)
	}
}
