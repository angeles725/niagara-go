package searchdoor

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// courseFiles orders each area like pathlib's sorted(rglob("*.java")): by path
// parts, and on Windows case-folded as WindowsPath.__lt__ compares them, so a
// lower-case name that folds first sorts before an upper-case one there.
func TestCourseFilesOrderIsPathlibOrder(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"course/Zeta.java", "course/alpha.java", "course/Lab/B.java", "course/lab2/a.java"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("class X {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, f := range courseFiles(root) {
		got = append(got, f.rel)
	}
	want := []string{"course/Lab/B.java", "course/Zeta.java", "course/alpha.java", "course/lab2/a.java"}
	if runtime.GOOS == "windows" {
		want = []string{"course/alpha.java", "course/Lab/B.java", "course/lab2/a.java", "course/Zeta.java"}
	}
	if !slices.Equal(got, want) {
		t.Errorf("courseFiles order = %q, want %q", got, want)
	}
}
