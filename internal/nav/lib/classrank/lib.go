package classrank

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// The library half of the package (decision D29): Source, Open, Rank,
// QueryTerms, Scope and FormatRow print nothing, take explicit paths instead
// of the nav runtime, and FormatRow yields the exact line `rank` prints, so
// the command and nsearch show the same text. Progress lines, staleness
// warnings and the absent-xref NOTE belong to the command (internal/nav/rankcmd).

// The index file names under indexes/.
const (
	ClassIndexFile = "class-index.json"
	XrefIndexFile  = "xref-index.json"
)

// Source is the files a Ranker is read from.
type Source struct {
	ClassIndex string // class-index.json
	XrefIndex  string // xref-index.json; when it is not a regular file, no importer prior
	Cache      string // the prepared-ranker cache (cache.go); "" never reads nor writes one
}

// SourceOf is the Source of a module-navigator base directory: the absolute
// indexes/class-index.json and indexes/xref-index.json and the cache
// indexes/.mnav-cache/class-rank.gob.
func SourceOf(baseDir string) Source {
	abs := func(name string) string {
		p := core.IndexPath(baseDir, name)
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}
	return Source{ClassIndex: abs(ClassIndexFile), XrefIndex: abs(XrefIndexFile),
		Cache: filepath.Join(core.IndexPath(baseDir, cacheDir), cacheName)}
}

// Row is one ranked class: its best-scoring top-level entry.
type Row struct {
	Class  string
	Score  float64
	Module string // "None" for a null module, as the output prints it
	Path   string
}

// FormatRow is the "rank\tscore\tclass\tmodule\tpath" line of `rank` (no
// newline); rank counts from 1 and the score has 3 decimals.
func FormatRow(rank int, r Row) string {
	return fmt.Sprintf("%d\t%s\t%s\t%s\t%s", rank, core.Fixed(r.Score, 3), r.Class, r.Module, r.Path)
}

// Rank returns every class sharing a term with question, best first: classes
// named in the question, then BM25 x importer prior (rank_all). Callers cut it.
func (r *Ranker) Rank(question string) []Row {
	results := r.rankAll(question)
	rows := make([]Row, len(results))
	for k, x := range results {
		rows[k] = Row{Class: x.name, Score: x.score, Module: r.module(x.doc), Path: r.st.Paths[x.doc]}
	}
	return rows
}

// QueryTerms returns the stemmed terms Rank searches for question, in order.
func (r *Ranker) QueryTerms(question string) []string { return r.terms.queryTerms(question) }

// Scope is the counted description of what Rank searches (the miss scope line).
func (r *Ranker) Scope() string { return r.scope() }

// Meta is the class index _meta the ranker was built from (nil when absent).
func (r *Ranker) Meta() *index.Object { return r.meta }

// isFile is os.path.isfile.
func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// ClassTables is (data["classes"], data["_meta"] or None) of a class index;
// ok is false when it has no classes table.
func ClassTables(data *index.Object) (classes, meta *index.Object, ok bool) {
	if v, _ := data.Get("classes"); v != nil {
		classes, _ = v.(*index.Object)
	}
	if v, _ := data.Get("_meta"); index.Truthy(v) {
		meta, _ = v.(*index.Object)
	}
	return classes, meta, classes != nil
}

// ImportedByTable is the class_imported_by table of an xref index (nil when
// absent or null); ok is false when it is present but not an object.
func ImportedByTable(xref *index.Object) (importedBy *index.Object, ok bool) {
	v, present := xref.Get("class_imported_by")
	if !present || v == nil {
		return nil, true
	}
	importedBy, ok = v.(*index.Object)
	return importedBy, ok
}

// loadObject is json.load of an index whose top-level value must be an object.
func loadObject(path, what string) (*index.Object, error) {
	v, err := index.LoadPython(path)
	if err == nil {
		if o, ok := v.(*index.Object); ok {
			return o, nil
		}
		err = fmt.Errorf("the top-level JSON value is not an object")
	}
	return nil, fmt.Errorf("%s is unreadable: %s (%v)", what, path, err)
}

// Loader is one load of a Source: the cache key is taken before the index
// files are read, so a ranker built from files that changed meanwhile is not
// cached. The nav command (rankcmd) reads the files itself, to print its
// progress lines, and hands the decoded tables to Build.
type Loader struct {
	src    Source
	key    cacheKey
	cached bool
}

// NewLoader fingerprints src's index files. Class indexes under cacheMinBytes,
// and a Source without a Cache path, are never cached.
func NewLoader(src Source) *Loader {
	key := keyOf(src)
	return &Loader{src: src, key: key, cached: src.Cache != "" && key.Class.Size >= cacheMinBytes}
}

// Cached is the ranker stored in the cache under the loader's key, else nil.
func (l *Loader) Cached() *Ranker {
	if !l.cached {
		return nil
	}
	return readCache(l.src.Cache, l.key)
}

// Build builds the ranker of the decoded tables (ClassTables, ImportedByTable)
// and refreshes the cache when it is used and neither index changed since
// NewLoader.
func (l *Loader) Build(classes, importedBy, meta *index.Object) *Ranker {
	st := build(classes, importedBy)
	r, err := newRanker(st, meta)
	if err != nil { // a freshly built state is always well formed
		panic(err)
	}
	if l.cached && l.key == keyOf(l.src) {
		writeCache(l.src.Cache, l.key, st, meta)
	}
	return r
}

// Open is the nav command's load without the runtime: the cached ranker of
// src, else one built from its index files (then cached). It prints nothing;
// an error is the message `rank` would print (an index missing, unreadable or
// without its table).
func Open(src Source) (*Ranker, error) {
	l := NewLoader(src)
	if r := l.Cached(); r != nil {
		return r, nil
	}
	if !isFile(src.ClassIndex) {
		return nil, fmt.Errorf("class index not found: %s", src.ClassIndex)
	}
	data, err := loadObject(src.ClassIndex, "class index")
	if err != nil {
		return nil, err
	}
	classes, meta, ok := ClassTables(data)
	if !ok {
		return nil, fmt.Errorf("class index has no 'classes' table: %s", src.ClassIndex)
	}
	var importedBy *index.Object
	if isFile(src.XrefIndex) {
		xref, err := loadObject(src.XrefIndex, "xref index")
		if err != nil {
			return nil, err
		}
		if importedBy, ok = ImportedByTable(xref); !ok {
			return nil, fmt.Errorf("xref index has no 'class_imported_by' table: %s", src.XrefIndex)
		}
	}
	return l.Build(classes, importedBy, meta), nil
}
