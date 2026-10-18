package helpbuild

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The port of tools/build_class_index.py: indexes/class-index.json (the
// javadoc HTML class list merged with every .bajadoc class under devguide/
// and guides/) and, when jdk/ exists, indexes/jdk-class-index.json (the
// .bajadoc classes under jdk/). Keys are simple names; a simple name shared
// by several classes keeps the preferred one (javax.baja.* first, then
// javadoc-backed, then by package and path) and lists the rest under
// "others". Both files are json.dumps(indent=2), ensure_ascii on.

// IndexResult is what one S8 or S9 index builder printed.
type IndexResult struct {
	Log    []string // stdout lines
	Stderr []string // stderr lines (the S9 builders' warnings)
	Status int      // exit status (inheritance_builder: 1 when bajadoc-clean/ is missing)
	// ParseFailures are the .bajadoc files that stayed unparseable after
	// sanitizing (bajadoc_xml.PARSE_FAILURES, which the script never prints),
	// in walk order, each once.
	ParseFailures []string
}

func (r *IndexResult) out(format string, args ...any) {
	r.Log = append(r.Log, fmt.Sprintf(format, args...))
}

// classLI is build_class_index._LI.
var classLI = pyre.MustCompile(`<li><a href="([^"]+)" title="(class|interface|enum|annotation) in `+
	`([^"]+)">(?:<span class="\w+">)?([^<]+)(?:</span>)?</a></li>`, 0)

// classKinds is build_class_index._KIND (bajadoc_xml kind -> index type).
var classKinds = map[string]string{"class": "class", "interface": "interface", "enum": "enum",
	"annotation_type": "annotation"}

// classXMLDirs is build_class_index.XML_DIRS.
var classXMLDirs = []string{"devguide", "guides"}

// classEntry is one class-index entry; Module is written only when not
// empty ("module" in entry).
type classEntry struct {
	Path, Package, Type, Module string
}

// namedClass is one (simple name, entry) pair of html_entries/xml_entries.
type namedClass struct {
	Name  string
	Entry classEntry
}

// classHTMLEntries is html_entries: every class the javadoc
// allclasses-noframe.html lists, in page order.
func classHTMLEntries(baseDir string) ([]namedClass, error) {
	path := pycompat.Join(baseDir, "bajadoc", "allclasses-noframe.html")
	if !isRegular(path) {
		return nil, nil
	}
	page, err := readPage(path)
	if err != nil {
		return nil, err
	}
	var out []namedClass
	for _, m := range classLI.FindIter([]rune(page)) {
		out = append(out, namedClass{m.Group(4), classEntry{Path: m.Group(1), Package: m.Group(3), Type: m.Group(2)}})
	}
	return out, nil
}

// classXMLEntries is xml_entries: every class of every .bajadoc under the
// dirs of baseDir, the path stored as "../<dir>/<file>" (index_paths.sibling
// of the root-relative path). Unparseable files are added to res. The files
// are parsed concurrently and taken in walk order, so the entries, the
// failures and the first error are the sequential ones.
func classXMLEntries(baseDir string, dirs []string, res *IndexResult) ([]namedClass, error) {
	var out []namedClass
	for _, d := range dirs {
		root := pycompat.Join(baseDir, d)
		if !isDir(root) {
			continue
		}
		var paths, rels []string
		bajadocWalk(root, func(path, rel string) { paths, rels = append(paths, path), append(rels, rel) })
		for i, f := range parseBajadocFiles(paths) {
			if f.err != nil {
				return out, f.err
			}
			if f.failed {
				res.addParseFailure(paths[i])
			}
			for _, h := range f.classes {
				e := classEntry{Path: "../" + d + "/" + rels[i], Package: h.Pkg, Type: classKinds[h.Kind], Module: h.Module}
				out = append(out, namedClass{h.Name, e})
			}
		}
	}
	return out, nil
}

func (r *IndexResult) addParseFailure(path string) {
	for _, p := range r.ParseFailures {
		if p == path {
			return
		}
	}
	r.ParseFailures = append(r.ParseFailures, path)
}

// classRankLess is the order of build_class_index._rank: javax.baja.*
// first, then javadoc pages before sibling-tree files, then package, path.
func classRankLess(a, b classEntry) bool {
	ab, bb := !strings.HasPrefix(a.Package, "javax.baja"), !strings.HasPrefix(b.Package, "javax.baja")
	if ab != bb {
		return !ab
	}
	as, bs := strings.HasPrefix(a.Path, "../"), strings.HasPrefix(b.Path, "../")
	if as != bs {
		return !as
	}
	if a.Package != b.Package {
		return a.Package < b.Package
	}
	return a.Path < b.Path
}

// mergeClasses is build_class_index.merge: one entry per qualified name
// (the first seen, taking the module of a later one when it has none),
// grouped by simple name, each group ranked; the key keeps the first and
// "others" the rest. Byte order is code-point order for UTF-8 strings.
func mergeClasses(entries []namedClass) pyjson.Value {
	byQualified := map[string]*namedClass{}
	var order []*namedClass
	for _, nc := range entries {
		q := nc.Entry.Package + "." + nc.Name
		have := byQualified[q]
		if have == nil {
			c := nc
			byQualified[q] = &c
			order = append(order, &c)
		} else if nc.Entry.Module != "" && have.Entry.Module == "" {
			have.Entry.Module = nc.Entry.Module // javadoc page + its module
		}
	}
	groups := map[string][]classEntry{}
	var names []string
	for _, nc := range order {
		if _, ok := groups[nc.Name]; !ok {
			names = append(names, nc.Name)
		}
		groups[nc.Name] = append(groups[nc.Name], nc.Entry)
	}
	sort.Strings(names)
	index := pyjson.NewObject()
	for _, name := range names {
		ranked := groups[name]
		sort.SliceStable(ranked, func(i, j int) bool { return classRankLess(ranked[i], ranked[j]) })
		primary := classEntryValue(ranked[0])
		if len(ranked) > 1 {
			others := make([]pyjson.Value, len(ranked)-1)
			for i, e := range ranked[1:] {
				others[i] = classEntryValue(e)
			}
			primary.Set("others", pyjson.Value{Kind: pyjson.Array, A: others})
		}
		index.Set(name, primary)
	}
	return index
}

func classEntryValue(e classEntry) pyjson.Value {
	v := pyjson.ObjectOf(pyjson.Field{Key: "path", Value: pyjson.String(e.Path)},
		pyjson.Field{Key: "package", Value: pyjson.String(e.Package)},
		pyjson.Field{Key: "type", Value: pyjson.String(e.Type)})
	if e.Module != "" {
		v.Set("module", pyjson.String(e.Module))
	}
	return v
}

// countClasses is build_class_index.count_classes.
func countClasses(index pyjson.Value) int {
	n := 0
	for _, k := range index.Keys {
		n++
		if o, ok := index.M[k].Get("others"); ok {
			n += len(o.A)
		}
	}
	return n
}

// BuildClassIndex is build_class_index.main() with baseDir as its argument:
// indexes/class-index.json, then indexes/jdk-class-index.json when jdk/ is
// a folder. err != nil is where Python raises (an unreadable page or
// .bajadoc file, a failed write: traceback, exit 1), with the lines printed
// before it in the result.
func BuildClassIndex(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	html, err := classHTMLEntries(baseDir)
	if err != nil {
		return res, err
	}
	xml, err := classXMLEntries(baseDir, classXMLDirs, res)
	if err != nil {
		return res, err
	}
	if err := writeClassIndex(res, baseDir, "class-index.json", mergeClasses(append(html, xml...))); err != nil {
		return res, err
	}
	if isDir(pycompat.Join(baseDir, "jdk")) {
		jdk, err := classXMLEntries(baseDir, []string{"jdk"}, res)
		if err != nil {
			return res, err
		}
		if err := writeClassIndex(res, baseDir, "jdk-class-index.json", mergeClasses(jdk)); err != nil {
			return res, err
		}
	}
	return res, nil
}

// writeClassIndex is build_class_index._write.
func writeClassIndex(res *IndexResult, baseDir, name string, index pyjson.Value) error {
	out := pycompat.Join(baseDir, "indexes", name)
	if err := os.MkdirAll(pycompat.Join(baseDir, "indexes"), 0o777); err != nil {
		return err
	}
	if err := WriteJSON(out, index, DumpIndent2); err != nil {
		return err
	}
	res.out("%s: %d names, %d classes -> %s", name, len(index.Keys), countClasses(index), out)
	return nil
}
