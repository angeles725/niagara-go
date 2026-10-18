package inventory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatsCacheKeyedBySizeAndMtime: a cached summary is reused while the
// file keeps its size and mtime, recomputed when either changes, and a
// summary without counts is not reused for a request that needs them.
func TestStatsCacheKeyedBySizeAndMtime(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(dir, ".mnav-cache", statsCacheName)); err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	write(t, p, `{"_meta": {"a": 9}}`) // same size, mtime restored: the cache answers
	os.Chtimes(p, stamp, stamp)
	c = openStatsCache(dir)
	if got := string(c.summary(p, false).Meta); got != `{"a": 1}` {
		t.Errorf("same size and mtime: meta %q, want the cached one", got)
	}
	if s := c.summary(p, true); !s.Full || string(s.Meta) != `{"a": 9}` {
		t.Errorf("a full request over a prefix-only entry is recomputed: %+v", s)
	}

	write(t, p, `{"_meta": {"a": 10}}`)
	c = openStatsCache(dir)
	if got := string(c.summary(p, false).Meta); got != `{"a": 10}` {
		t.Errorf("changed size: meta %q", got)
	}
}
