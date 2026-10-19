package helpbuild

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// With TestS8UnreadableFrozen (the frozen Python output): an input that
// cannot be read (a .bajadoc for class-index, a bajadoc-clean/ page for
// method-index) stops the builder with an error, and the indexes it would
// have written are not replaced by partial ones: the previous files stay
// byte for byte and no temp file is left.
func TestS8UnreadableInputKeepsThePreviousIndexes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not make a file unreadable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	for _, c := range []struct {
		mode, unreadable string
		names            []string
	}{
		{"class", filepath.Join("devguide", "baja", "javax", "baja", "sys", "BIObject.bajadoc"),
			[]string{"class-index.json", "jdk-class-index.json"}},
		{"method", filepath.Join("bajadoc-clean", "javax", "baja", "sys", "BMode.txt"), []string{"method-index.json"}},
	} {
		mode, names := c.mode, c.names
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			copyTree(t, filepath.Join("testdata", "s8", "root"), base)
			for _, n := range names {
				writeFile(t, filepath.Join(base, "indexes", n), []byte("previous "+n))
			}
			p := filepath.Join(base, c.unreadable)
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
			if res, err := goIndexBuild(mode, base); err == nil {
				t.Fatalf("%s built over an unreadable input: %+v", mode, res)
			}
			for _, n := range names {
				if got := readText(t, filepath.Join(base, "indexes", n)); got != "previous "+n {
					t.Errorf("%s replaced by %.60q", n, got)
				}
			}
			noDotEntries(t, filepath.Join(base, "indexes"))
		})
	}
}
