package core

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// The decoded class-index cache, indexes/.mnav-cache/class-index.gob, beside
// the call-graph cache (lib/callgraph/cache.go) and with its rules: parsing
// the 34 MB class-index.json costs about 0.4 s on every class, hierarchy and
// callers run. The cache keeps the decoded tree under the index's
// symlink-resolved path and statcache.Stamp (size, mtime, ctime, inode): an
// index whose path or stamp differs, or a cache that cannot be read or is malformed, is ignored
// (the JSON is decoded and the cache rewritten atomically by WriteGob). A
// cache that cannot be written is skipped silently, so the output never
// depends on it. Load prints the same stderr on a hit and on a miss.
//
// The tree is flattened: every distinct string (keys, string values and the
// text of numbers) once in Strings, cut by Ends, and the values in document
// order as an op stream (classOp): decoding the cache is a few allocations
// plus the rebuilt objects, no JSON scanning.
//
// Indexes under classCacheMinBytes are not cached: decoding them costs less
// than the cache round trip, and the small read-only test fixtures stay
// untouched.

const (
	classCacheName   = "class-index.gob"
	classCacheFormat = 2 // 2: the key is the whole statcache.Stamp (ctime and inode too)
)

// classCacheMinBytes is the index size from which the class index is cached.
var classCacheMinBytes int64 = 1 << 20

// The op stream: each value is one op, kind in the low 3 bits and its
// argument above them (a string number, or an element count followed by the
// elements; an object element is a key string number then the value).
const (
	opNull = iota
	opFalse
	opTrue
	opString
	opNumber
	opObject
	opArray
	opBits = 3
)

type classCacheKey struct {
	Format int
	Path   string // the index path, absolute with symlinks resolved
	statcache.Stamp
}

type classCacheFile struct {
	Key     classCacheKey
	Strings string
	Ends    []int
	Ops     []int
}

// ClassIndexCachePath is the class-index cache file of a base directory.
func ClassIndexCachePath(baseDir string) string {
	return filepath.Join(IndexPath(baseDir, ".mnav-cache"), classCacheName)
}

// classCacheKeyOf fingerprints the index at path; ok is false when it is
// not a regular file of at least classCacheMinBytes.
func classCacheKeyOf(path string) (classCacheKey, bool) {
	s, ok := statcache.Regular(path, classCacheMinBytes)
	if !ok {
		return classCacheKey{}, false
	}
	return classCacheKey{Format: classCacheFormat, Path: ResolvedPath(path), Stamp: s}, true
}

// loadClassIndex is LoadJSON for the class index at path, through the cache.
func (r *Runtime) loadClassIndex(path, what string) *index.Object {
	key, ok := classCacheKeyOf(path)
	if !ok {
		return r.LoadJSON(path, what)
	}
	cache := ClassIndexCachePath(r.BaseDir)
	if o := readClassCache(cache, key); o != nil {
		return o
	}
	o := r.LoadJSON(path, what)
	if o != nil {
		// A tree decoded after the index changed is not stored.
		if now, ok := classCacheKeyOf(path); ok && now == key {
			writeClassCache(cache, key, o)
		}
	}
	return o
}

func readClassCache(path string, key classCacheKey) *index.Object {
	var cf classCacheFile
	if !statcache.ReadGob(path, &cf) || cf.Key != key {
		return nil
	}
	o, _ := fromClassCache(&cf).(*index.Object)
	return o
}

// fromClassCache rebuilds the tree of a decoded cache file, nil when the
// file is inconsistent.
func fromClassCache(cf *classCacheFile) any {
	strs := make([]string, len(cf.Ends))
	start := 0
	for i, end := range cf.Ends {
		if end < start || end > len(cf.Strings) {
			return nil
		}
		strs[i], start = cf.Strings[start:end], end
	}
	if start != len(cf.Strings) {
		return nil
	}
	d := opDecoder{ops: cf.Ops, strs: strs}
	v, ok := d.value()
	if !ok || d.pos != len(d.ops) {
		return nil
	}
	return v
}

type opDecoder struct {
	ops  []int
	strs []string
	pos  int
}

func (d *opDecoder) next() (int, bool) {
	if d.pos >= len(d.ops) {
		return 0, false
	}
	d.pos++
	return d.ops[d.pos-1], true
}

func (d *opDecoder) str(n int) (string, bool) {
	if n < 0 || n >= len(d.strs) {
		return "", false
	}
	return d.strs[n], true
}

func (d *opDecoder) value() (any, bool) {
	op, ok := d.next()
	if !ok {
		return nil, false
	}
	arg := op >> opBits
	switch op & (1<<opBits - 1) {
	case opNull:
		return nil, true
	case opFalse:
		return false, true
	case opTrue:
		return true, true
	case opString:
		return d.str(arg)
	case opNumber:
		s, ok := d.str(arg)
		return json.Number(s), ok
	case opArray:
		if arg < 0 || arg > len(d.ops)-d.pos {
			return nil, false
		}
		list := make([]any, arg)
		for i := range list {
			if list[i], ok = d.value(); !ok {
				return nil, false
			}
		}
		return list, true
	case opObject:
		if arg < 0 || arg > len(d.ops)-d.pos {
			return nil, false
		}
		o := index.NewObject()
		for i := 0; i < arg; i++ {
			kn, ok := d.next()
			if !ok {
				return nil, false
			}
			k, ok := d.str(kn)
			if !ok || o.Has(k) {
				return nil, false
			}
			v, ok := d.value()
			if !ok {
				return nil, false
			}
			o.Set(k, v)
		}
		return o, true
	}
	return nil, false
}

// writeClassCache stores o, or nothing when it holds a value the op stream
// does not represent.
func writeClassCache(path string, key classCacheKey, o *index.Object) bool {
	e := opEncoder{ids: map[string]int{}}
	if !e.value(o) {
		return false
	}
	cf := classCacheFile{Key: key, Strings: e.strs.String(), Ends: e.ends, Ops: e.ops}
	return WriteGob(path, &cf)
}

type opEncoder struct {
	ids  map[string]int
	strs strings.Builder
	ends []int
	ops  []int
}

func (e *opEncoder) id(s string) int {
	if n, ok := e.ids[s]; ok {
		return n
	}
	e.strs.WriteString(s)
	e.ends = append(e.ends, e.strs.Len())
	e.ids[s] = len(e.ends) - 1
	return len(e.ends) - 1
}

func (e *opEncoder) op(kind, arg int) { e.ops = append(e.ops, arg<<opBits|kind) }

func (e *opEncoder) value(v any) bool {
	switch x := v.(type) {
	case nil:
		e.op(opNull, 0)
	case bool:
		if x {
			e.op(opTrue, 0)
		} else {
			e.op(opFalse, 0)
		}
	case string:
		e.op(opString, e.id(x))
	case json.Number:
		e.op(opNumber, e.id(string(x)))
	case []any:
		e.op(opArray, len(x))
		for _, el := range x {
			if !e.value(el) {
				return false
			}
		}
	case *index.Object:
		keys, vals := x.Keys(), x.Values()
		e.op(opObject, len(keys))
		for i, k := range keys {
			e.ops = append(e.ops, e.id(k))
			if !e.value(vals[i]) {
				return false
			}
		}
	default:
		return false
	}
	return true
}
