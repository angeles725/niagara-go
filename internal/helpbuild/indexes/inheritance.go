package indexes

import (
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The port of tools/inheritance_builder.py: indexes/inheritance.json, per
// class of bajadoc-clean/*.txt its extends, implements, subclasses and
// package, read from the header (the lines before the first empty one). A
// simple name shared by several classes keeps the preferred one (javax.baja.*
// first, then by package) and the others are keyed by qualified name. The
// file is json.dumps(indent=2, sort_keys=True, ensure_ascii=False).

// inheritInfo is inheritance_builder.parse_txt_file's dict; a nil pointer
// is Python's None.
type inheritInfo struct {
	Name, Package, Extends *string
	Implements, Subclasses []string
}

// splitHeaderList is `[s.strip() for s in raw.split(',') if s.strip()]`.
func splitHeaderList(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = pycompat.Strip(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// erasure is a type name without its type arguments, entities decoded:
// "IterableCursor<E>", "IterableCursor&lt;E&gt;" and a cut "java.lang.Enum&lt;"
// are "IterableCursor" and "java.lang.Enum". A name without "<" is unchanged.
func erasure(name string) string {
	name = pyhtml.Unescape(name)
	if i := strings.IndexByte(name, '<'); i >= 0 {
		return pycompat.Strip(name[:i])
	}
	return name
}

// parseInheritTxt is inheritance_builder.parse_txt_file; err is the text of
// the exception main() catches (an OSError or a UnicodeDecodeError). Unlike
// Python the class name and its extends are erasures (erasure): a generic
// class is keyed "IterableCursor", as the chain commands look it up, not
// "IterableCursor&lt;E&gt;".
func parseInheritTxt(path string) (*inheritInfo, error) {
	info := &inheritInfo{Implements: []string{}, Subclasses: []string{}}
	set := func(p **string, s string) { *p = &s }
	setName := func(p **string, s string) { s = erasure(s); *p = &s }
	err := pyTextLines(path, func(line string) bool {
		line = strings.TrimRight(line, "\n")
		if line == "" {
			return false // an empty line ends the header
		}
		// The prefixes are ASCII, so Python's code-point slices are byte slices.
		switch {
		case strings.HasPrefix(line, "CLASS: "):
			setName(&info.Name, pycompat.Strip(line[7:]))
		case strings.HasPrefix(line, "INTERFACE: "):
			setName(&info.Name, pycompat.Strip(line[11:]))
		case strings.HasPrefix(line, "ENUM: "):
			setName(&info.Name, pycompat.Strip(line[6:]))
		case strings.HasPrefix(line, "ANNOTATION TYPE: "):
			setName(&info.Name, pycompat.Strip(line[17:]))
		case strings.HasPrefix(line, "PACKAGE: "):
			set(&info.Package, pycompat.Strip(line[9:]))
		case strings.HasPrefix(line, "EXTENDS: "):
			if s := pycompat.Strip(line[9:]); s != "" {
				setName(&info.Extends, s)
			} else {
				info.Extends = nil
			}
		case strings.HasPrefix(line, "IMPLEMENTS: "):
			if raw := pycompat.Strip(line[12:]); raw != "" {
				info.Implements = splitHeaderList(raw)
			}
		case strings.HasPrefix(line, "SUBCLASSES: "):
			if raw := pycompat.Strip(line[12:]); raw != "" {
				info.Subclasses = splitHeaderList(raw)
			}
		}
		return true
	})
	return info, err
}

// cleanTxtFiles is the sorted os.walk of bajadoc-clean/: every *.txt, the
// folders and files of each folder in sorted order.
func cleanTxtFiles(cleanDir string) []string {
	var files []string
	pycompat.Walk(cleanDir, func(dir string, dirs, names []string) []string {
		sort.Strings(dirs)
		sort.Strings(names)
		for _, name := range names {
			if strings.HasSuffix(name, ".txt") {
				files = append(files, pycompat.Join(dir, name))
			}
		}
		return dirs
	})
	return files
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func optValue(p *string) pyjson.Value {
	if p == nil {
		return pyjson.Value{Kind: pyjson.Null}
	}
	return pyjson.String(*p)
}

// BuildInheritance is inheritance_builder.main() with baseDir as its
// argument. A file that cannot be read or decoded, or holds no class line,
// is listed under "Errors"; err != nil is where Python raises (a failed
// write: traceback, exit 1), with the lines printed before it.
func BuildInheritance(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	cleanDir := pycompat.Join(baseDir, "bajadoc-clean")
	outDir := pycompat.Join(baseDir, "indexes")
	if !pycompat.IsDir(cleanDir) {
		res.out("ERROR: bajadoc-clean/ not found at %s", cleanDir)
		res.Status = 1
		return res, nil
	}
	files := cleanTxtFiles(cleanDir)
	res.out("Found %d .txt files in bajadoc-clean/", len(files))
	var parsed []*inheritInfo
	var errs []string
	nameCount := map[string]int{}
	for _, path := range files {
		info, err := parseInheritTxt(path)
		switch {
		case err != nil:
			errs = append(errs, path+": "+err.Error())
		case info.Name == nil:
			errs = append(errs, "No CLASS/INTERFACE in: "+path)
		default:
			parsed = append(parsed, info)
			nameCount[*info.Name]++
		}
	}
	type pick struct {
		notJavax bool
		pkg      string
		chosen   *string
	}
	preferred := map[string]pick{}
	for _, info := range parsed {
		pkg := strOr(info.Package)
		r := pick{!strings.HasPrefix(pkg, "javax.baja"), pkg, info.Package}
		cur, ok := preferred[*info.Name]
		if !ok || r.notJavax != cur.notJavax && !r.notJavax || r.notJavax == cur.notJavax && r.pkg < cur.pkg {
			preferred[*info.Name] = r
		}
	}
	sorted := append([]*inheritInfo(nil), parsed...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if *a.Name != *b.Name {
			return *a.Name < *b.Name
		}
		return strOr(a.Package) < strOr(b.Package)
	})
	entries := map[string]*inheritInfo{}
	for _, info := range sorted {
		key := *info.Name
		if nameCount[key] > 1 && strOr(info.Package) != "" && (preferred[key].chosen == nil || *preferred[key].chosen != *info.Package) {
			key = *info.Package + "." + *info.Name
		}
		entries[key] = info
	}
	duplicates := 0
	for _, c := range nameCount {
		if c > 1 {
			duplicates++
		}
	}
	res.out("Parsed %d entries from %d files (%d duplicate names resolved with FQN)", len(entries), len(parsed), duplicates)
	if len(errs) > 0 {
		res.out("Errors (%d):", len(errs))
		for _, e := range errs {
			res.out("  %s", e)
		}
	}
	// sort_keys=True for the top level too: byte order is code-point order.
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	inheritance := pyjson.NewObject()
	for _, k := range keys {
		info := entries[k]
		// sort_keys=True: the members are set in sorted order here.
		inheritance.Set(k, pyjson.ObjectOf(pyjson.Field{Key: "extends", Value: optValue(info.Extends)},
			pyjson.Field{Key: "implements", Value: pyjson.StringList(info.Implements)},
			pyjson.Field{Key: "package", Value: optValue(info.Package)},
			pyjson.Field{Key: "subclasses", Value: pyjson.StringList(info.Subclasses)}))
	}
	if !pycompat.IsDir(outDir) {
		if err := os.MkdirAll(outDir, 0o777); err != nil {
			return res, err
		}
	}
	outPath := pycompat.Join(outDir, "inheritance.json")
	if err := pyfs.WriteJSON(outPath, inheritance, pyfs.DumpIndent2Unicode); err != nil {
		return res, err
	}
	st, err := os.Stat(outPath)
	if err != nil {
		return res, err
	}
	res.out("Wrote %s (%.2f MB)", outPath, float64(st.Size())/(1024.0*1024.0))
	res.Log = append(res.Log, "", "--- Verification ---") // print('\n--- Verification ---')
	verifyInheritance(res, entries)
	return res, nil
}

// verifyInheritance is inheritance_builder.verify_samples.
func verifyInheritance(res *IndexResult, data map[string]*inheritInfo) {
	if bc, ok := data["BComponent"]; ok {
		res.out("BComponent:")
		res.out("  extends: %s", pycompat.OptStr(bc.Extends))
		res.out("  implements: %s", pycompat.Repr(bc.Implements))
		res.out("  subclasses count: %d", len(bc.Subclasses))
		res.out("  package: %s", pycompat.OptStr(bc.Package))
	} else {
		res.out("WARNING: BComponent not found!")
	}
	if bas, ok := data["BAbstractService"]; ok {
		res.out("BAbstractService:")
		res.out("  extends: %s", pycompat.OptStr(bas.Extends))
		res.out("  subclasses count: %d", len(bas.Subclasses))
	} else {
		res.out("WARNING: BAbstractService not found!")
	}
	if bio, ok := data["BIObject"]; ok {
		res.out("BIObject:")
		res.out("  extends: %s", pycompat.OptStr(bio.Extends))
		res.out("  subclasses count: %d", len(bio.Subclasses))
		ifaces := 0
		for _, s := range bio.Subclasses {
			if strings.HasPrefix(s, "BI") {
				ifaces++
			}
		}
		res.out("  of which interfaces (BI*): %d", ifaces)
	} else {
		res.out("WARNING: BIObject not found!")
	}
	var withExtends, withSubs, withImpl int
	for _, v := range data {
		if v.Extends != nil && *v.Extends != "" {
			withExtends++
		}
		if len(v.Subclasses) > 0 {
			withSubs++
		}
		if len(v.Implements) > 0 {
			withImpl++
		}
	}
	res.Log = append(res.Log, "", "Stats:")
	res.out("  Total entries: %d", len(data))
	res.out("  With extends: %d", withExtends)
	res.out("  With subclasses: %d", withSubs)
	res.out("  With implements: %d", withImpl)
}
