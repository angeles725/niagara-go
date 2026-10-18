package helpbuild

import (
	"database/sql"
	"path"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The 'indexed' checks of `coverage --doc-jars` (doc_coverage.Indexed):
// lazy views over one data root's indexes, each loaded on first use.

// FulltextDocs returns the "<source>/<path>" key of every document of the
// up-to-date full-text index of root (fulltext_index.open_index(root).docs()).
// The CLI passes an opener that rebuilds a stale index, as open_index does;
// FulltextDocsAt reads one database.
type FulltextDocs func(root string) (map[string]bool, error)

// FulltextDocsAt is FtsIndex.docs() of the full-text database at path.
func FulltextDocsAt(path string) (map[string]bool, error) {
	db, err := index.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT source, path FROM docs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var src, p sql.NullString
		if err := rows.Scan(&src, &p); err != nil {
			return nil, err
		}
		out[src.String+"/"+p.String] = true
	}
	return out, rows.Err()
}

// indexedViews is doc_coverage.Indexed.
type indexedViews struct {
	root     string
	docs     FulltextDocs
	classes  map[bool]map[string]bool // jdk -> "pkg.Name"
	sources  map[string]bool
	tocs     map[string]bool
	fulltext map[string]bool
	devToc   *bool
}

func newIndexedViews(root string, docs FulltextDocs) *indexedViews {
	return &indexedViews{root: root, docs: docs, classes: map[bool]map[string]bool{}}
}

// recordOf is a record of an index that must be an object.
func recordOf(name, key string, v any) (*index.Object, error) {
	o, ok := v.(*index.Object)
	if !ok {
		return nil, &IndexShapeError{name, "the record of " + key + " is not an object"}
	}
	return o, nil
}

// classSet is Indexed.classes(jdk): "<package>.<key>" of every entry of
// class-index.json (jdk-class-index.json) and of its "others".
func (x *indexedViews) classSet(jdk bool) (map[string]bool, error) {
	if s, ok := x.classes[jdk]; ok {
		return s, nil
	}
	name := "class-index.json"
	if jdk {
		name = "jdk-class-index.json"
	}
	idx, err := loadReportIndex(x.root, name, false)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for i, key := range idx.Keys() {
		entry, err := recordOf(name, key, idx.Values()[i])
		if err != nil {
			return nil, err
		}
		all := []*index.Object{entry}
		if others, ok := entry.Get("others"); ok {
			list, ok := others.([]any)
			if !ok {
				return nil, &IndexShapeError{name, "the others of " + key + " are not a list"}
			}
			for _, o := range list {
				e, err := recordOf(name, key, o)
				if err != nil {
					return nil, err
				}
				all = append(all, e)
			}
		}
		for _, e := range all {
			pkg, ok := e.Get("package")
			if !ok {
				return nil, &IndexShapeError{name, "a record of " + key + " has no package"}
			}
			out[pycompat.Str(pkg)+"."+key] = true
		}
	}
	x.classes[jdk] = out
	return out, nil
}

// pyNormpath is posixpath.normpath on every OS: only "/" separates (a
// backslash or a drive letter is part of a name), two leading slashes are
// kept, three or more become one.
func pyNormpath(p string) string {
	if p == "" {
		return "."
	}
	clean := path.Clean(p)
	if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		return "/" + clean
	}
	return clean
}

// sourceSet is Indexed.sources(): normpath("source/" + path) of every
// source-index.json record.
func (x *indexedViews) sourceSet() (map[string]bool, error) {
	if x.sources != nil {
		return x.sources, nil
	}
	const name = "source-index.json"
	idx, err := loadReportIndex(x.root, name, false)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for i, key := range idx.Keys() {
		data, err := recordOf(name, key, idx.Values()[i])
		if err != nil {
			return nil, err
		}
		p, ok := data.Get("path")
		if !ok {
			return nil, &IndexShapeError{name, "the record of " + key + " has no path"}
		}
		s, ok := p.(string)
		if !ok {
			return nil, &IndexShapeError{name, "the path of " + key + " is not a string"}
		}
		out[pyNormpath(pycompat.PosixJoin("source", s))] = true
	}
	x.sources = out
	return out, nil
}

// pyTruthy is bool(v) of a decoded JSON value.
func pyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case *index.Object:
		return x.Len() > 0
	}
	f, ok := index.Float(v)
	return !ok || f != 0
}

// guideTocs is Indexed.guide_tocs(): the guides-index.json folders whose
// record has a true "toc".
func (x *indexedViews) guideTocs() (map[string]bool, error) {
	if x.tocs != nil {
		return x.tocs, nil
	}
	const name = "guides-index.json"
	idx, err := loadReportIndex(x.root, name, false)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for i, key := range idx.Keys() {
		d, err := recordOf(name, key, idx.Values()[i])
		if err != nil {
			return nil, err
		}
		if toc, _ := d.Get("toc"); pyTruthy(toc) {
			out[key] = true
		}
	}
	x.tocs = out
	return out, nil
}

func (x *indexedViews) fulltextDocs() (map[string]bool, error) {
	if x.fulltext != nil {
		return x.fulltext, nil
	}
	if x.docs == nil {
		return nil, errNoFulltext
	}
	docs, err := x.docs(x.root)
	if err != nil {
		return nil, err
	}
	if docs == nil {
		docs = map[string]bool{}
	}
	x.fulltext = docs
	return docs, nil
}

// devguideToc is Indexed.devguide_toc(): devguide-toc.json has sections.
func (x *indexedViews) devguideToc() (bool, error) {
	if x.devToc == nil {
		idx, err := loadReportIndex(x.root, "devguide-toc.json", false)
		if err != nil {
			return false, err
		}
		sections, _ := idx.Get("sections")
		ok := pyTruthy(sections)
		x.devToc = &ok
	}
	return *x.devToc, nil
}

// check is Indexed.check: the extracted file rel of kind is searchable.
func (x *indexedViews) check(kind, rel string) (bool, error) {
	path := pycompat.Join(x.root, rel)
	switch kind {
	case "javadoc-class":
		q := strings.ReplaceAll(dropRunes(dropPrefixRunes(rel, len("bajadoc/")), len(".html")), "/", ".")
		classes, err := x.classSet(false)
		return classes[q], err
	case "bajadoc-xml", "jdk-xml":
		classes, err := x.classSet(kind == "jdk-xml")
		if err != nil {
			return false, err
		}
		parsed, _, err := readBajadocClasses(path)
		if err != nil || len(parsed) == 0 {
			return false, err
		}
		for _, c := range parsed {
			if !classes[c.Pkg+"."+c.Name] {
				return false, nil
			}
		}
		return true, nil
	case "devguide-page", "guide-page":
		folder := "devguide"
		if kind == "guide-page" {
			folder = "guides"
		}
		page := dropPrefixRunes(rel, len(folder)+1)
		return isRegular(pycompat.Join(x.root, folder+"-clean",
			CleanTextName(page, pycompat.Join(x.root, folder)))), nil
	case "guide-toc":
		tocs, err := x.guideTocs()
		return tocs[strings.Split(rel, "/")[1]], err
	case "devguide-toc":
		return x.devguideToc()
	case "java":
		sources, err := x.sourceSet()
		return sources[rel], err
	case "example":
		docs, err := x.fulltextDocs()
		return docs[rel], err
	}
	return false, nil
}

// dropPrefixRunes is s[n:] in code points.
func dropPrefixRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return ""
	}
	return string(r[n:])
}
