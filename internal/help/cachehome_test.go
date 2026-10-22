package help

import (
	"os"
	"testing"
)

// TestMain points XDG_CACHE_HOME at a scratch folder for the whole package:
// a search writes the fingerprint memo (fts.CachedFingerprint) and, for a
// read-only data root, the index into the user cache, which a test must not
// leave in the real ~/.cache. A test that sets its own keeps it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nh-cache-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CACHE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
