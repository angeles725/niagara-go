package modjar

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The cache follows the stats cache of module-nav (internal/nav/inventory/statscache.go):
// one gob file of {absolute path: entry} kept under each jar's size and mtime (ns), written
// back atomically with core.WriteGob (mode 0600). A jar whose size or mtime changed is read
// again; a jar that changed while it was read, or failed to open or read, is not cached;
// "no such member" is (it is a property of the bytes). The key is not resolved through
// symlinks: on a slow file system that costs an lstat per path component, and a stat of
// the path itself is all a hit needs. A run is taken as the whole set of jars of each
// directory it read: Save drops the entries of those directories that it did not read
// (removed jars) without a stat per entry. A cache that cannot be read is
// ignored and one that cannot be written is skipped, so the output never depends on it.
//
// The file lives in $XDG_CACHE_HOME (else ~/.cache)/niagara-go/modjar/, one per member,
// never inside a repository. NIAGARA_GO_MODJAR_CACHE=off (0, no, false) turns it off, and
// a test binary never uses it.

const (
	cacheFormat = 1
	envCache    = "NIAGARA_GO_MODJAR_CACHE"
)

type cacheEntry struct {
	Size, MtimeNs int64
	Data          []byte
	MetaInf       []string
	NoMember      bool
}

type cacheFile struct {
	Format  int
	Member  string
	Entries map[string]cacheEntry
}

// Cache holds the cached answers for one member. A disabled cache (OpenCache with no
// directory) reads every jar and saves nothing.
type Cache struct {
	member, path string
	mu           sync.Mutex
	entries      map[string]cacheEntry
	dirty        bool
	seen         map[string]bool // keys read in this run
	seenDirs     map[string]bool // their directories
}

// OpenCache loads the cache of member under dir; dir "" gives a disabled cache.
func OpenCache(dir, member string) *Cache {
	c := &Cache{member: member, entries: map[string]cacheEntry{}, seen: map[string]bool{}, seenDirs: map[string]bool{}}
	if dir == "" {
		return c
	}
	sum := sha1.Sum([]byte(member))
	c.path = filepath.Join(dir, hex.EncodeToString(sum[:])[:16]+".gob")
	data, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f cacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&f) == nil && f.Format == cacheFormat &&
		f.Member == member && f.Entries != nil {
		c.entries = f.Entries
	}
	return c
}

// ReadAll is the package ReadAll through the cache.
func (c *Cache) ReadAll(paths []string, workers int) []Result {
	return pool(paths, workers, c.read)
}

func (c *Cache) read(path string) Result {
	if c.path == "" {
		return ReadMember(path, c.member)
	}
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return ReadMember(path, c.member) // reports the error the open meets
	}
	key, err := filepath.Abs(path)
	if err != nil {
		return ReadMember(path, c.member)
	}
	c.mu.Lock()
	e, ok := c.entries[key]
	c.seen[key], c.seenDirs[filepath.Dir(key)] = true, true
	c.mu.Unlock()
	if ok && e.Size == st.Size() && e.MtimeNs == st.ModTime().UnixNano() {
		r := Result{Path: path, Data: e.Data, MetaInf: e.MetaInf}
		if e.NoMember {
			r.Data, r.MetaInf = nil, nil
			r.Err = fmt.Errorf("%s: %w: %s", path, ErrNoMember, c.member)
		}
		return r
	}
	r := ReadMember(path, c.member)
	noMember := errors.Is(r.Err, ErrNoMember)
	if r.Err != nil && !noMember {
		return r
	}
	if st2, err := os.Stat(path); err == nil && st2.Size() == st.Size() && st2.ModTime().Equal(st.ModTime()) {
		c.mu.Lock()
		c.entries[key] = cacheEntry{st.Size(), st.ModTime().UnixNano(), r.Data, r.MetaInf, noMember}
		c.dirty = true
		c.mu.Unlock()
	}
	return r
}

// Save writes the cache back when an answer was added or a jar of a directory read in this
// run is gone. A write failure is returned for the caller to ignore: the results
// never depend on the cache.
func (c *Cache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		return nil
	}
	for k := range c.entries {
		if c.seenDirs[filepath.Dir(k)] && !c.seen[k] {
			delete(c.entries, k)
			c.dirty = true
		}
	}
	if !c.dirty {
		return nil
	}
	if !core.WriteGob(c.path, cacheFile{cacheFormat, c.member, c.entries}) {
		return fmt.Errorf("modjar: cannot write cache %s", c.path)
	}
	c.dirty = false
	return nil
}

// ReadAllCached is ReadAll through the default cache (DefaultDir), saved afterwards.
func ReadAllCached(paths []string, member string, workers int) []Result {
	c := OpenCache(DefaultDir(), member)
	res := c.ReadAll(paths, workers)
	_ = c.Save() // the results are the answer either way
	return res
}

// DefaultDir is the cache directory, "" when the cache is off.
func DefaultDir() string {
	exe, err := os.Executable()
	return defaultDir(err != nil || strings.HasSuffix(strings.TrimSuffix(filepath.Base(exe), ".exe"), ".test"))
}

// defaultDir is DefaultDir for a run that is (or not) a test binary.
func defaultDir(testBinary bool) string {
	switch strings.ToLower(pycompat.Strip(os.Getenv(envCache))) {
	case "off", "0", "no", "false":
		return ""
	}
	if testBinary {
		return ""
	}
	home := os.Getenv("XDG_CACHE_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return ""
		}
		home = filepath.Join(h, ".cache")
	}
	return filepath.Join(home, "niagara-go", "modjar")
}
