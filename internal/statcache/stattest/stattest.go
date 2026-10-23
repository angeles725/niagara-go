// Package stattest is a test helper for the stat-keyed caches: EditKeepingStamp
// rewrites a file the way cp -p, rsync -a, tar x or touch -r leave it, with the
// size and modification time it had before.
package stattest

import (
	"os"
	"testing"
	"time"
)

// EditKeepingStamp replaces the content of path in place with data (which must
// be as long as the old content) and sets its modification time back. The
// kernel stamps the ctime from a coarse clock, so a rewrite within the tick of
// the last change keeps the ctime a cache may have recorded: where the system
// reports ctimes (Linux, as statcache.Sys), the edit is repeated until the ctime differs,
// whatever the tick, and fails the test if it never does.
func EditKeepingStamp(t testing.TB, path string, data []byte) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != int64(len(data)) {
		t.Fatalf("EditKeepingStamp: %s has %d bytes, the edit %d", path, st.Size(), len(data))
	}
	before, hasCtime := ctimeOf(st)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(time.Millisecond) {
		if err := os.WriteFile(path, data, st.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, st.ModTime(), st.ModTime()); err != nil {
			t.Fatal(err)
		}
		st2, err := os.Stat(path)
		if err != nil || st2.Size() != st.Size() || !st2.ModTime().Equal(st.ModTime()) {
			t.Fatalf("EditKeepingStamp: %s did not keep its size and mtime (%v)", path, err)
		}
		if ctime, _ := ctimeOf(st2); !hasCtime || ctime != before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("EditKeepingStamp: the ctime of %s never changed", path)
		}
	}
}
