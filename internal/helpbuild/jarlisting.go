package helpbuild

import (
	"crypto/sha1"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Cached zip listings of the jars of a Niagara install
// (niagara_help_lib/jar_listing.py).
//
// A Listings keeps, per jar, the file entries (name, size, CRC-32) keyed by
// the jar's absolute path, size and mtime: an unchanged jar is never
// reopened, a changed or new one is listed again. Compiled .class entries
// are only counted. The cache file is the one Python writes,
// <repo>/.cache/jar-listings/<sha1(abspath)[:16]>.json, in Python's compact
// json.dumps: each tool reads the other's cache and writes the same bytes.

// CacheVersion is the cache format version (jar_listing.CACHE_VERSION).
const CacheVersion = 1

// CacheDir is the cache folder under the repository root.
var CacheDir = filepath.Join(".cache", "jar-listings")

// pyAbspath is os.path.abspath. On POSIX it keeps the exactly-two leading
// slashes posixpath.normpath keeps and filepath.Clean folds (a relative
// path is joined to the working directory, which never starts with "//").
func pyAbspath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err == nil && runtime.GOOS != "windows" && strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		abs = "/" + abs
	}
	return abs, err
}

// CachePath is the cache file of one install (jar_listing.cache_path).
func CachePath(repoRoot, installDir string) (string, error) {
	abs, err := pyAbspath(installDir)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(abs))
	return filepath.Join(repoRoot, CacheDir, hex.EncodeToString(sum[:])[:16]+".json"), nil
}

// Entry is one file entry of a jar listing.
type Entry struct {
	Name string
	Size uint64
	CRC  uint32
}

// Listing is a jar's file entries in central-directory order (directories
// dropped, duplicates kept) and the number of .class entries.
type Listing struct {
	Entries []Entry
	Classes int
}

// readListing is jar_listing.read_listing with the reason of a nil listing.
func readListing(jarPath string) (*Listing, error) {
	infos, err := zipfile.ReadCentralDir(jarPath)
	if err != nil {
		return nil, err
	}
	l := &Listing{Entries: []Entry{}}
	for _, i := range infos {
		switch {
		case strings.HasSuffix(i.Name, "/"):
		case strings.HasSuffix(i.Name, ".class"):
			l.Classes++
		default:
			l.Entries = append(l.Entries, Entry{Name: i.Name, Size: i.Size, CRC: i.CRC})
		}
	}
	return l, nil
}

// ReadListing lists a jar, or returns nil when it cannot be read (not a
// zip, corrupt, I/O error), as read_listing returns None.
func ReadListing(jarPath string) *Listing {
	l, _ := readListing(jarPath)
	return l
}

// value is the listing as Python's dict ({"entries": [[name, size, crc],
// ...], "classes": n}), or null for an unreadable jar.
func (l *Listing) value() pyjson.Value {
	if l == nil {
		return pyjson.Value{}
	}
	entries := make([]pyjson.Value, len(l.Entries))
	for i, e := range l.Entries {
		entries[i] = pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{
			pyjson.String(e.Name), uintValue(e.Size), pyjson.IntValue(int64(e.CRC))}}
	}
	v := pyjson.NewObject()
	v.Set("entries", pyjson.Value{Kind: pyjson.Array, A: entries})
	v.Set("classes", pyjson.IntValue(int64(l.Classes)))
	return v
}

func uintValue(n uint64) pyjson.Value {
	return pyjson.Value{Kind: pyjson.Int, I: new(big.Int).SetUint64(n)}
}

// listingOf converts a cached listing back; ok is false when its shape is
// not one read_listing writes (Python would fail later on it: here the jar
// is listed again).
func listingOf(v pyjson.Value) (l *Listing, ok bool) {
	if v.Kind == pyjson.Null {
		return nil, true
	}
	entries, _ := v.Get("entries")
	classes, _ := v.Get("classes")
	if entries.Kind != pyjson.Array || classes.Kind != pyjson.Int || !classes.I.IsInt64() || classes.I.Sign() < 0 {
		return nil, false
	}
	l = &Listing{Entries: make([]Entry, 0, len(entries.A)), Classes: int(classes.I.Int64())}
	for _, e := range entries.A {
		if e.Kind != pyjson.Array || len(e.A) != 3 || e.A[0].Kind != pyjson.Str ||
			e.A[1].Kind != pyjson.Int || !e.A[1].I.IsUint64() ||
			e.A[2].Kind != pyjson.Int || !e.A[2].I.IsUint64() || e.A[2].I.Uint64() > 0xFFFFFFFF {
			return nil, false
		}
		l.Entries = append(l.Entries, Entry{Name: e.A[0].S, Size: e.A[1].I.Uint64(), CRC: uint32(e.A[2].I.Uint64())})
	}
	return l, true
}

// Listings are jar listings read through an optional on-disk cache
// (jar_listing.Listings). Path "" keeps them in memory only.
type Listings struct {
	Path  string
	Reads int // jars actually opened (cache misses)
	jars  pyjson.Value
	dirty bool
}

// NewListings loads the cache at path; a missing, damaged or other-version
// cache starts empty (it is rebuilt, never trusted).
func NewListings(path string) *Listings {
	ls := &Listings{Path: path, jars: pyjson.NewObject()}
	if path == "" {
		return ls
	}
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return ls
	}
	raw, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(raw) {
		return ls
	}
	data, err := pyjson.Loads([]rune(string(raw)))
	if err != nil || data.Kind != pyjson.Object {
		return ls
	}
	if version, _ := data.Get("version"); !pyjson.Equal(version, pyjson.IntValue(CacheVersion)) {
		return ls
	}
	if jars, _ := data.Get("jars"); jars.Kind == pyjson.Object {
		ls.jars = jars
	}
	return ls
}

// Listing returns the listing of jarPath (nil when unreadable), from the
// cache when the jar's size and mtime are unchanged. A jar that cannot be
// stat'ed is nil and not cached.
func (ls *Listings) Listing(jarPath string) *Listing {
	key, err := pyAbspath(jarPath)
	if err != nil {
		return nil
	}
	st, err := os.Stat(key)
	if err != nil {
		return nil
	}
	stamp := pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{
		pyjson.IntValue(st.Size()), pyjson.IntValue(st.ModTime().UnixNano())}}
	if hit, ok := ls.jars.Get(key); ok && hit.Truthy() {
		if s, _ := hit.Get("stamp"); pyjson.Equal(s, stamp) {
			cached, _ := hit.Get("listing")
			if l, ok := listingOf(cached); ok {
				return l
			}
		}
	}
	l := ReadListing(key)
	ls.Reads++
	rec := pyjson.NewObject()
	rec.Set("stamp", stamp)
	rec.Set("listing", l.value())
	ls.jars.Set(key, rec)
	ls.dirty = true
	return l
}

// Names returns the file entry names (no .class) of jarPath; ok is false
// when the jar is unreadable.
func (ls *Listings) Names(jarPath string) (names []string, ok bool) {
	l := ls.Listing(jarPath)
	if l == nil {
		return nil, false
	}
	names = make([]string, len(l.Entries))
	for i, e := range l.Entries {
		names[i] = e.Name
	}
	return names, true
}

// Save writes the cache atomically when something was listed anew.
func (ls *Listings) Save() error {
	if ls.Path == "" || !ls.dirty {
		return nil
	}
	data := pyjson.NewObject()
	data.Set("version", pyjson.IntValue(CacheVersion))
	data.Set("jars", ls.jars)
	if err := WriteJSON(ls.Path, data, DumpCompact); err != nil {
		return err
	}
	ls.dirty = false
	return nil
}
