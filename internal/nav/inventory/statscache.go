package inventory

import (
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// The stats summary cache, indexes/.mnav-cache/stats.gob: Python's stats reads
// every JSON index whole (about 300 MB, 7 to 30 s) to print a few _meta
// counters. The port keeps, per index file of statsCacheMinBytes or more, what
// stats needs from it (summary) under the file's statcache.Stamp (size, mtime,
// ctime, inode); a file whose stamp changed is read again. A cache that cannot be read is
// ignored and one that cannot be written is skipped, so the output never
// depends on it. Smaller files (and the read-only test fixtures) are read
// every time.

const (
	statsCacheName   = "stats.gob"
	statsCacheFormat = 2 // 2: the key is the whole statcache.Stamp (ctime and inode too)
)

// statsCacheMinBytes is the index size from which a summary is cached.
var statsCacheMinBytes int64 = 1 << 20

type statsEntry struct {
	statcache.Stamp
	Summary summary
}

type statsCacheFile struct {
	Format  int
	Entries map[string]statsEntry
}

// statsCache holds the cached summaries of one stats run.
type statsCache struct {
	path    string
	entries map[string]statsEntry
	dirty   bool
}

func openStatsCache(indexDir string) *statsCache {
	c := &statsCache{path: filepath.Join(indexDir, ".mnav-cache", statsCacheName), entries: map[string]statsEntry{}}
	var f statsCacheFile
	if statcache.ReadGob(c.path, &f) && f.Format == statsCacheFormat && f.Entries != nil {
		c.entries = f.Entries
	}
	return c
}

// summary returns the summary of the JSON file at path; full asks for the
// counts that need the whole document.
func (c *statsCache) summary(path string, full bool) summary {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return summary{}
	}
	stamp := statcache.Of(st)
	key := core.ResolvedPath(path)
	cacheable := stamp.Size >= statsCacheMinBytes
	if e, ok := c.entries[key]; ok && cacheable && e.Stamp == stamp &&
		(e.Summary.Full || !full) {
		return e.Summary
	}
	s := summarize(path, full)
	if cacheable && statcache.Same(path, stamp) { // a file changed while summarized is not stored
		c.entries[key] = statsEntry{stamp, s}
		c.dirty = true
	}
	return s
}

// save writes the cache back when a summary was added (statcache.WriteGob:
// atomically, mode 0600), dropping the entries of files that no longer exist.
func (c *statsCache) save() {
	if !c.dirty {
		return
	}
	for k := range c.entries {
		if !statcache.Stat(k).Exists() {
			delete(c.entries, k)
		}
	}
	statcache.WriteGob(c.path, statsCacheFile{statsCacheFormat, c.entries})
	c.dirty = false
}
