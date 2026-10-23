package callgraph

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/statcache/stattest"
)

// TestCacheKeySeesAnEditThatKeepsSizeAndMtime: cp -p, rsync -a and touch -r keep
// the index's size and mtime; the cache key still changes.
func TestCacheKeySeesAnEditThatKeepsSizeAndMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime and inode are read on Linux only (statcache.Sys)")
	}
	defer func(n int64) { cacheMinBytes = n }(cacheMinBytes)
	cacheMinBytes = 1
	p := filepath.Join(t.TempDir(), "callgraph-index.json")
	if err := os.WriteFile(p, []byte(`{"a": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := NewCache(p, "c.gob").key
	stattest.EditKeepingStamp(t, p, []byte(`{"a": 9}`))
	if after := NewCache(p, "c.gob").key; after == before {
		t.Fatalf("key %+v unchanged by an edit keeping size and mtime", after)
	}
}
