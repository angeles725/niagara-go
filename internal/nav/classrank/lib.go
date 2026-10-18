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
// warnings and the absent-xref NOTE belong to the command (Load, Cmd).

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
	return Source{ClassIndex: abs(classIndex), XrefIndex: abs(xrefIndex),
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

// isFile is os.path.isfile.
func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// classTables is (data["classes"], data["_meta"] or None) of a class index;
// ok is false when it has no classes table.
func classTables(data *index.Object) (classes, meta *index.Object, ok bool) {
	if v, _ := data.Get("classes"); v != nil {
		classes, _ = v.(*index.Object)
	}
	if v, _ := data.Get("_meta"); truthy(v) {
		meta, _ = v.(*index.Object)
	}
	return classes, meta, classes != nil
}

// importedByTable is the class_imported_by table of an xref index (nil when
// absent or null); ok is false when it is present but not an object.
func importedByTable(xref *index.Object) (importedBy *index.Object, ok bool) {
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

// fromTables builds the ranker of the decoded tables and refreshes src's
// cache when cached and neither index changed since key was taken.
func fromTables(src Source, key cacheKey, cached bool, classes, importedBy, meta *index.Object) *Ranker {
	st := build(classes, importedBy)
	r, err := newRanker(st, meta)
	if err != nil { // a freshly built state is always well formed
		panic(err)
	}
	if cached && key == keyOf(src) {
		writeCache(src.Cache, key, st, meta)
	}
	return r
}

// Open is Load without the nav runtime: the cached ranker of src, else one
// built from its index files (then cached). It prints nothing; an error is
// the message `rank` would print (an index missing, unreadable or without its
// table).
func Open(src Source) (*Ranker, error) {
	key := keyOf(src)
	cached := src.Cache != "" && key.Class.Size >= cacheMinBytes
	if cached {
		if r := readCache(src.Cache, key); r != nil {
			return r, nil
		}
	}
	if !isFile(src.ClassIndex) {
		return nil, fmt.Errorf("class index not found: %s", src.ClassIndex)
	}
	data, err := loadObject(src.ClassIndex, "class index")
	if err != nil {
		return nil, err
	}
	classes, meta, ok := classTables(data)
	if !ok {
		return nil, fmt.Errorf("class index has no 'classes' table: %s", src.ClassIndex)
	}
	var importedBy *index.Object
	if isFile(src.XrefIndex) {
		xref, err := loadObject(src.XrefIndex, "xref index")
		if err != nil {
			return nil, err
		}
		if importedBy, ok = importedByTable(xref); !ok {
			return nil, fmt.Errorf("xref index has no 'class_imported_by' table: %s", src.XrefIndex)
		}
	}
	return fromTables(src, key, cached, classes, importedBy, meta), nil
}
