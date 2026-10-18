package navbuild

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The cross-reference index (build_xref.py): class imports and their reverse,
// module dependencies both ways, and the importers of classes outside the
// corpus, from the import statements of every top-level class-index entry,
// indexes/xref-index.json.

// XrefFile is the index the xref builder writes.
const XrefFile = "xref-index.json"

// Xref is what build_xref computes.
type Xref struct {
	ClassImports, ExternalImportedBy                          *orderedLists // insertion order, values sorted
	ClassImportedBy, ModuleDeps                               *orderedLists // keys and values sorted
	ModuleDependedBy                                          *orderedLists
	FilesParsed, FilesFailed, ImportStatements, SkippedStdlib int
	ResolvedImports, WildcardImports, WildcardResolved        int
	Elapsed                                                   time.Duration
}

// headerLines is how many lines build_xref scans for imports.
const headerLines = 200

// ComputeXref is build_xref over the class-index entries, reading sources
// under organized.
func ComputeXref(entries []classIndexEntry, organized string) *Xref {
	packageClasses := newOrderedLists()
	type info struct{ name, module string }
	fqcnInfo := map[string]info{}
	classModules := map[string]map[string]bool{}
	var top []classIndexEntry
	for _, e := range entries {
		if e.Outer {
			continue
		}
		if e.Package != "" {
			packageClasses.add(e.Package, e.Name)
		}
		fqcn := e.Name
		if e.Package != "" {
			fqcn = e.Package + "." + e.Name
		}
		fqcnInfo[fqcn] = info{e.Name, e.Module}
		if classModules[e.Name] == nil {
			classModules[e.Name] = map[string]bool{}
		}
		classModules[e.Name][e.Module] = true
		top = append(top, e)
	}

	x := &Xref{ClassImports: newOrderedLists(), ExternalImportedBy: newOrderedLists()}
	scans := scanImports(top, organized)
	for n, e := range top {
		switch {
		case e.Path == "":
			continue
		case scans[n].failed:
			x.FilesFailed++
			continue
		}
		x.FilesParsed++
		imported := map[string]bool{}
		var importedOrder []string
		add := func(c string) {
			if !imported[c] {
				imported[c] = true
				importedOrder = append(importedOrder, c)
			}
		}
		for _, m := range scans[n].imports {
			x.ImportStatements++
			importedFQCN := m.path
			if m.static {
				if i := strings.LastIndexByte(m.path, '.'); i >= 0 {
					importedFQCN = m.path[:i]
				}
			}
			external := !strings.HasSuffix(m.path, ".*")
			if (strings.HasPrefix(m.path, "java.") || strings.HasPrefix(m.path, "javax.")) &&
				!strings.HasPrefix(m.path, "javax.baja.") {
				x.SkippedStdlib++
				if external {
					x.ExternalImportedBy.add(importedFQCN, e.Name)
				}
				continue
			}
			if !external {
				x.WildcardImports++
				pkg := strings.TrimSuffix(m.path, ".*")
				for _, c := range packageClasses.vals[pkg] {
					add(c)
					x.WildcardResolved++
				}
				continue
			}
			parts := strings.Split(m.path, ".")
			resolve := func(fqcn, fallback string) {
				if inf, ok := fqcnInfo[fqcn]; ok {
					add(inf.name)
				} else if classModules[fallback] != nil {
					add(fallback)
				} else {
					return
				}
				x.ResolvedImports++
				external = false
			}
			if m.static && len(parts) >= 2 {
				resolve(strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-2])
			} else {
				resolve(m.path, parts[len(parts)-1])
			}
			if external {
				x.ExternalImportedBy.add(importedFQCN, e.Name)
			}
		}
		delete(imported, e.Name)
		for _, c := range importedOrder {
			if imported[c] {
				x.ClassImports.add(e.Name, c)
			}
		}
	}
	x.ClassImports.sortValues()
	x.ExternalImportedBy.sortValues()

	// The reverse and module maps were filled from sets in Python: their keys
	// are sorted since module-navigator fix/deterministic-xref-base-dir.
	x.ClassImportedBy, x.ModuleDeps, x.ModuleDependedBy = newOrderedLists(), newOrderedLists(), newOrderedLists()
	for _, importer := range x.ClassImports.keys {
		for _, importee := range x.ClassImports.vals[importer] {
			x.ClassImportedBy.add(importee, importer)
			for imp := range classModules[importer] {
				for dep := range classModules[importee] {
					if imp != dep {
						x.ModuleDeps.add(imp, dep)
						x.ModuleDependedBy.add(dep, imp)
					}
				}
			}
		}
	}
	for _, o := range []*orderedLists{x.ClassImportedBy, x.ModuleDeps, x.ModuleDependedBy} {
		sort.Strings(o.keys)
		o.sortValues()
	}
	return x
}

// fileImports is the import statements of one source, or failed when it is
// not a readable file.
type fileImports struct {
	failed  bool
	imports []importMatch
}

// scanImports reads the import header of every entry's source (four workers);
// the result is indexed like entries.
func scanImports(entries []classIndexEntry, organized string) []fileImports {
	out := make([]fileImports, len(entries))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(4, runtime.GOMAXPROCS(0)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range next {
				full := entries[n].Path // os.path.join keeps an absolute second part
				if !filepath.IsAbs(full) {
					full = filepath.Join(organized, full)
				}
				content, err := pycompat.ReadTextReplace(full)
				if !isFile(full) || err != nil {
					out[n].failed = true
					continue
				}
				out[n].imports = findImports(firstLines(content, headerLines))
			}
		}()
	}
	for n, e := range entries {
		if e.Path != "" {
			next <- n
		}
	}
	close(next)
	wg.Wait()
	return out
}

// firstLines is "\n".join(content.split("\n", n)[:n]): content up to its n-th "\n".
func firstLines(content string, n int) string {
	at := 0
	for i := 0; i < n; i++ {
		j := strings.IndexByte(content[at:], '\n')
		if j < 0 {
			return content
		}
		at += j + 1
	}
	return content[:at-1]
}

// importMatch is one RE_IMPORT match: static, and group 2.
type importMatch struct {
	static bool
	path   string
}

// findImports is RE_IMPORT.finditer(text) with
// RE_IMPORT = ^\s*import\s+(static\s+)?([\w.]+(?:\.\*)?)\s*; (re.MULTILINE):
// a match is tried at every line start; \s* may cross blank lines.
func findImports(text string) []importMatch {
	var out []importMatch
	for p := 0; p <= len(text); {
		if m, end, ok := matchImport(text, p); ok {
			out = append(out, m)
			p = end
		}
		j := strings.IndexByte(text[p:], '\n')
		if j < 0 {
			break
		}
		p += j + 1
	}
	return out
}

// matchImport tries RE_IMPORT at p. The static branch is tried first; when the
// rest fails after it, "static" is read as the start of the imported name, as
// the regex backtracks.
func matchImport(s string, p int) (importMatch, int, bool) {
	i := spanFunc(s, p, pycompat.IsSpace)
	if !strings.HasPrefix(s[i:], "import") {
		return importMatch{}, 0, false
	}
	i += len("import")
	j := spanFunc(s, i, pycompat.IsSpace)
	if j == i {
		return importMatch{}, 0, false
	}
	if strings.HasPrefix(s[j:], "static") {
		k := j + len("static")
		if m := spanFunc(s, k, pycompat.IsSpace); m > k {
			if path, end, ok := importPath(s, m); ok {
				return importMatch{true, path}, end, true
			}
		}
	}
	if path, end, ok := importPath(s, j); ok {
		return importMatch{false, path}, end, true
	}
	return importMatch{}, 0, false
}

// importPath matches ([\w.]+(?:\.\*)?)\s*; at i. Only two lengths of the
// greedy [\w.]+ can succeed: all of the run, or all but a final "." that
// starts ".*".
func importPath(s string, i int) (string, int, bool) {
	w := spanFunc(s, i, func(r rune) bool { return r == '.' || pycompat.IsWordRune(r) })
	if w == i {
		return "", 0, false
	}
	if e := spanFunc(s, w, pycompat.IsSpace); e < len(s) && s[e] == ';' {
		return s[i:w], e + 1, true
	}
	if w-1 > i && s[w-1] == '.' && w < len(s) && s[w] == '*' {
		if e := spanFunc(s, w+1, pycompat.IsSpace); e < len(s) && s[e] == ';' {
			return s[i : w+1], e + 1, true
		}
	}
	return "", 0, false
}

// Document is the xref-index.json text (compact).
func (x *Xref) Document(at time.Time) string {
	maxImports, sumImports := 0, 0
	for _, k := range x.ClassImports.keys {
		n := len(x.ClassImports.vals[k])
		maxImports = max(maxImports, n)
		sumImports += n
	}
	avg := integer(0) // Python's `else 0` is an int, so round() keeps it one
	if n := len(x.ClassImports.keys); n > 0 {
		avg = float(round1(float64(sumImports) / float64(n)))
	}
	maxImporters := 0
	for _, k := range x.ClassImportedBy.keys {
		maxImporters = max(maxImporters, len(x.ClassImportedBy.vals[k]))
	}
	meta := stamped([]Field{
		{"description", str("Cross-reference (import) index for Niagara N4 decompiled modules")},
		{"source_index", str(ClassIndexFile)},
		{"files_parsed", integer(x.FilesParsed)},
		{"files_failed", integer(x.FilesFailed)},
		{"total_import_statements", integer(x.ImportStatements)},
		{"skipped_stdlib", integer(x.SkippedStdlib)},
		{"resolved_imports", integer(x.ResolvedImports)},
		{"wildcard_imports", integer(x.WildcardImports)},
		{"wildcard_classes_resolved", integer(x.WildcardResolved)},
		{"classes_with_imports", integer(len(x.ClassImports.keys))},
		{"classes_imported_by_others", integer(len(x.ClassImportedBy.keys))},
		{"external_classes", integer(len(x.ExternalImportedBy.keys))},
		{"modules_with_deps", integer(len(x.ModuleDeps.keys))},
		{"modules_depended_on", integer(len(x.ModuleDependedBy.keys))},
		{"max_imports_per_class", integer(maxImports)},
		{"avg_imports_per_class", avg},
		{"max_importers_per_class", integer(maxImporters)},
		{"total_class_import_edges", integer(sumImports)},
		{"total_module_dep_edges", integer(x.ModuleDeps.total())},
	}, x.Elapsed, at)
	w := &jsonWriter{o: Compact}
	keys := []string{"_meta", "class_imports", "class_imported_by", "module_deps", "module_depended_by", "external_imported_by"}
	maps := []*orderedLists{nil, x.ClassImports, x.ClassImportedBy, x.ModuleDeps, x.ModuleDependedBy, x.ExternalImportedBy}
	w.object(0, len(keys), func(i int) string { return keys[i] }, func(i, level int) {
		if i == 0 {
			w.fields(level, meta)
			return
		}
		w.lists(level, maps[i])
	})
	return w.b.String()
}

// BuildXref reads <baseDir>'s class index and the sources under organized and
// writes <baseDir>/indexes/xref-index.json (build_xref without the summary).
func BuildXref(baseDir, organized string) (*Xref, error) {
	t0 := time.Now()
	entries, err := readClassIndex(baseDir)
	if err != nil {
		return nil, err
	}
	x := ComputeXref(entries, organized)
	x.Elapsed = time.Since(t0)
	if err := writeIndex(baseDir, XrefFile, x.Document(now())); err != nil {
		return nil, fmt.Errorf("writing %s: %w", XrefFile, err)
	}
	return x, nil
}
