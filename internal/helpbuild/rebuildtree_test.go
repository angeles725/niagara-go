package helpbuild

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Two runs that differ only in the run times build.log records must give
// the same tree observation (TestParityRebuildSynthetic flaked when a
// builder's "Done in N.Ns" differed by one digit), and build.log is put
// back after the walk.
func TestRebuildTreeLinesNormalizesBuildLogTimes(t *testing.T) {
	observe := func(log string) []string {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "build.log"), []byte(log))
		lines := rebuildTreeLines(t, root, false, false)
		if got, err := os.ReadFile(filepath.Join(root, "build.log")); err != nil || string(got) != log {
			t.Fatalf("build.log not restored: %q, %v", got, err)
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
