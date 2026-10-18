package helpbuild

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// Two runs that differ only in the run times build.log records must give
// the same tree observation (TestParityRebuildSynthetic flaked when a
// builder's "Done in N.Ns" differed by one digit), and build.log is put
// back after the walk with its text and its permission bits (0666 is one
// the umask would lower if the restore relied on the create mode).
func TestRebuildTreeLinesNormalizesBuildLogTimes(t *testing.T) {
	observe := func(log string) []string {
		root := t.TempDir()
		path := filepath.Join(root, "build.log")
		writeFile(t, path, []byte(log))
		if err := os.Chmod(path, 0o666); err != nil {
			t.Fatal(err)
		}
		lines := rebuildTreeLines(t, root, false, false)
		if got, err := os.ReadFile(path); err != nil || string(got) != log {
			t.Fatalf("build.log not restored: %q, %v", got, err)
		}
		if st, err := os.Lstat(path); err != nil || !st.Mode().IsRegular() {
			t.Fatalf("build.log restored as %v, %v", st, err)
		} else if runtime.GOOS != "windows" && st.Mode().Perm() != 0o666 {
			t.Fatalf("build.log restored with mode %o, want 666", st.Mode().Perm())
		}
		return lines
	}
	fast := observe("[bajadoc]\nDone in 0.1s\nTime:              1.2 s\n")
	slow := observe("[bajadoc]\nDone in 9.8s\nTime:              37.5 s\n")
	if !slices.Equal(fast, slow) {
		t.Errorf("run times leak into the comparison:\n fast %q\n slow %q", fast, slow)
	}
	if other := observe("[bajadoc]\nDone in 0.1s\nTime:              1.2 s\nextra\n"); slices.Equal(fast, other) {
		t.Error("a real build.log difference is hidden")
	}
}
