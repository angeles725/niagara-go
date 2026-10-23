package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/statcache"
	"github.com/angeles725/niagara-go/internal/statcache/stattest"
)

// TestStatsCacheKeyedByStamp: a cached summary is reused while the file keeps
// its statcache.Stamp, recomputed when it changes (size, or an edit that keeps
// size and mtime: cp -p, rsync -a, touch -r), and a summary without counts is
// not reused for a request that needs them.
func TestStatsCacheKeyedByStamp(t *testing.T) {
	defer func(n int64) { statsCacheMinBytes = n }(statsCacheMinBytes)
	statsCacheMinBytes = 0
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	write(t, p, `{"_meta": {"a": 1}}`)
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(p, stamp, stamp)
	c := openStatsCache(dir)
	if got := string(c.summary(p, false).Meta); got != `{"a": 1}` {
		t.Fatalf("first summary meta %q", got)
	}
	c.save()
	cache := filepath.Join(dir, ".mnav-cache", statsCacheName)
	var f statsCacheFile
	if !statcache.ReadGob(cache, &f) || len(f.Entries) != 1 {
		t.Fatalf("cache not written")
	}
	// Plant a marker in the cached entry: only a hit can answer it.
	for k, e := range f.Entries {
		e.Summary.Meta = []byte(`{"a": 7}`)
		f.Entries[k] = e
	}
	if !statcache.WriteGob(cache, f) {
		t.Fatal("cannot rewrite the cache")
	}
	c = openStatsCache(dir)
	if got := string(c.summary(p, false).Meta); got != `{"a": 7}` {
		t.Errorf("unchanged file: meta %q, want the cached one", got)
	}
	if s := c.summary(p, true); !s.Full || string(s.Meta) != `{"a": 1}` {
		t.Errorf("a full request over a prefix-only entry is recomputed: %+v", s)
	}
	c.save()

	if runtime.GOOS == "linux" { // ctime and inode are read on Linux only (statcache.Sys)
		stattest.EditKeepingStamp(t, p, []byte(`{"_meta": {"a": 9}}`))
		c = openStatsCache(dir)
		if got := string(c.summary(p, false).Meta); got != `{"a": 9}` {
			t.Errorf("same size and mtime, new content: meta %q, want the file's", got)
		}
	}

	write(t, p, `{"_meta": {"a": 10}}`)
	c = openStatsCache(dir)
	if got := string(c.summary(p, false).Meta); got != `{"a": 10}` {
		t.Errorf("changed size: meta %q", got)
	}
}
