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
// be as long as the old content) and sets its modification time back. It waits
// past the kernel's coarse timestamp tick first, so the edit's ctime differs
// from the one a cache recorded just before.
func EditKeepingStamp(t testing.TB, path string, data []byte) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != int64(len(data)) {
		t.Fatalf("EditKeepingStamp: %s has %d bytes, the edit %d", path, st.Size(), len(data))
	}
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, data, st.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	if st2, err := os.Stat(path); err != nil || st2.Size() != st.Size() || !st2.ModTime().Equal(st.ModTime()) {
		t.Fatalf("EditKeepingStamp: %s did not keep its size and mtime (%v)", path, err)
	}
}
