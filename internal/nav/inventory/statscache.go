package inventory

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
)

// The stats summary cache, indexes/.mnav-cache/stats.gob: Python's stats reads
// every JSON index whole (about 300 MB, 7 to 30 s) to print a few _meta
// counters. The port keeps, per index file of statsCacheMinBytes or more, what
// stats needs from it (summary) under the file's size and mtime (ns); a file
// whose size or mtime changed is read again. A cache that cannot be read is
// ignored and one that cannot be written is skipped, so the output never
// depends on it. Smaller files (and the read-only test fixtures) are read
// every time.

const (
	statsCacheName   = "stats.gob"
	statsCacheFormat = 1
)

// statsCacheMinBytes is the index size from which a summary is cached.
var statsCacheMinBytes int64 = 1 << 20

type statsEntry struct {
	Size, MtimeNs int64
	Summary       summary
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
	data, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f statsCacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&f) == nil && f.Format == statsCacheFormat && f.Entries != nil {
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
	key := realPath(path)
	cacheable := st.Size() >= statsCacheMinBytes
	if e, ok := c.entries[key]; ok && cacheable && e.Size == st.Size() && e.MtimeNs == st.ModTime().UnixNano() &&
		(e.Summary.Full || !full) {
		return e.Summary
	}
	s := summarize(path, full)
	if cacheable {
		if st2, err := os.Stat(path); err == nil && st2.Size() == st.Size() && st2.ModTime().Equal(st.ModTime()) {
			c.entries[key] = statsEntry{st.Size(), st.ModTime().UnixNano(), s}
			c.dirty = true
		}
	}
	return s
}

// realPath is the absolute path with symlinks resolved: the cache key, so a
// base directory reached through a link shares the entries of the real one.
func realPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// save writes the cache back when a summary was added (atomically; mode
// 0600), dropping the entries of files that no longer exist.
func (c *statsCache) save() {
	if !c.dirty {
		return
	}
	for k := range c.entries {
		if _, err := os.Stat(k); err != nil {
			delete(c.entries, k)
		}
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(statsCacheFile{statsCacheFormat, c.entries}) != nil {
		return
	}
	dir := filepath.Dir(c.path)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	f, err := os.CreateTemp(dir, statsCacheName+".*.tmp")
	if err != nil {
		return
	}
	_, werr := f.Write(buf.Bytes())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(f.Name(), c.path)
	}
	if werr != nil {
		os.Remove(f.Name())
	}
	c.dirty = false
}
