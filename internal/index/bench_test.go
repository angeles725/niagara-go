package index

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestLoadTiming reports how long the real module-navigator indexes take to
// load (set MNAV_INDEX_DIR to its indexes/ directory; skipped otherwise).
func TestLoadTiming(t *testing.T) {
	dir := os.Getenv("MNAV_INDEX_DIR")
	if dir == "" || testing.Short() {
		t.Skip("MNAV_INDEX_DIR not set")
	}
	for _, name := range []string{"module-inventory.json", "class-index.json", "method-index.json"} {
		best := time.Duration(1 << 62)
		for range 3 {
			start := time.Now()
			if _, err := LoadPython(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			best = min(best, time.Since(start))
			runtime.GC()
		}
		t.Logf("%s: best of 3 %v", name, best)
	}
}
