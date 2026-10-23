package bognav

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/slotomatic"
	"github.com/angeles725/niagara-go/internal/statcache"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The parsed type catalog of one decompiler dir, cached on disk: parsing the decompiled sources is
// most of a relays or slot run (about 0.3 s of 0.45 s on PANCCADIA, audit T28). The cache keeps a
// dir's parseFiles result under $XDG_CACHE_HOME (or ~/.cache)/niagara-go/bog-nav/<dir hash>.gob,
// keyed by the dir's path and, for every .java file in walk order, its path and statcache.Stamp
// (size, mtime, ctime, inode; Missing when it cannot be stat'ed): a changed (an edit that keeps
// size and mtime still changes the ctime), added or removed file, or a cache that cannot be read
// or is malformed, is a miss (the dir is parsed and the cache rewritten atomically by
// statcache.WriteGob). A parse during which the files changed is not stored, and a cache that
// cannot be written is skipped silently, so the output never depends on the cache.
// NIAGARA_CACHE_DIR=off (the bog-nav goldens set it, as the Python tool read it) turns the cache
// off.

const catalogCacheFormat = 2 // 2: the key is the whole statcache.Stamp (ctime and inode too)

type catalogFile struct {
	Path            string
	statcache.Stamp // statcache.Missing when the file cannot be stat'ed
}

type catalogKey struct {
	Format int
	Dir    string
	Files  []catalogFile
}

type catalogEntry struct {
	Key     string
	Type    slotomatic.Type
	Imports []string
	Source  string
}

type catalogCache struct {
	Key     catalogKey
	Entries []catalogEntry
}

// catalogCacheDir is the cache directory, "" when NIAGARA_CACHE_DIR turns the cache off or there
// is no cache home.
func catalogCacheDir() string {
	if textfold.Lower(os.Getenv("NIAGARA_CACHE_DIR")) == "off" {
		return ""
	}
	return statcache.UserDir("niagara-go", "bog-nav")
}

func catalogKeyOf(src string, paths []string) catalogKey {
	k := catalogKey{Format: catalogCacheFormat, Dir: src, Files: make([]catalogFile, len(paths))}
	for i, st := range statcache.Files(paths, 1) {
		k.Files[i] = catalogFile{Path: paths[i], Stamp: st}
	}
	return k
}

func (k catalogKey) equal(o catalogKey) bool {
	return k.Format == o.Format && k.Dir == o.Dir && slices.Equal(k.Files, o.Files)
}

// cachedParseDir is parseDir through the on-disk cache.
func cachedParseDir(src string) ([]string, map[string]*classRec) {
	dir := catalogCacheDir()
	paths := javaFiles(src)
	if dir == "" {
		return parseFiles(src, paths)
	}
	sum := sha1.Sum([]byte(core.ResolvedPath(src)))
	cache := filepath.Join(dir, hex.EncodeToString(sum[:])[:16]+".gob")
	key := catalogKeyOf(src, paths)
	if keys, recs, ok := readCatalogCache(cache, key); ok {
		return keys, recs
	}
	keys, recs := parseFiles(src, paths)
	if now := javaFiles(src); slices.Equal(now, paths) && catalogKeyOf(src, now).equal(key) {
		cf := catalogCache{Key: key, Entries: make([]catalogEntry, len(keys))}
		for i, k := range keys {
			r := recs[k]
			cf.Entries[i] = catalogEntry{Key: k, Type: r.Type, Imports: r.imports, Source: r.source}
		}
		statcache.WriteGob(cache, &cf)
	}
	return keys, recs
}

func readCatalogCache(path string, key catalogKey) ([]string, map[string]*classRec, bool) {
	var cf catalogCache
	if !statcache.ReadGob(path, &cf) || !cf.Key.equal(key) {
		return nil, nil, false
	}
	keys := make([]string, 0, len(cf.Entries))
	recs := make(map[string]*classRec, len(cf.Entries))
	for _, e := range cf.Entries {
		if _, dup := recs[e.Key]; dup {
			return nil, nil, false
		}
		recs[e.Key] = &classRec{Type: e.Type, imports: e.Imports, source: e.Source}
		keys = append(keys, e.Key)
	}
	return keys, recs, true
}
