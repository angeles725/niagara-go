package navbuild

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/javatext"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The class index (build_class_index.py): every type declared in a vineflower
// .java file of the corpus, indexes/class-index.json.

// ClassIndexFile is the index the class-index builder writes.
const ClassIndexFile = "class-index.json"

// ClassEntry is one class-index entry. Outer is "" for a top-level type
// (outer_class null).
type ClassEntry struct {
	Package, Module, Kind string
	Extends               optString
	Implements, Modifiers []string
	Lines                 int
	Path                  string
	ZKM                   bool
	InnerClasses          []string
	Outer                 string
}

// ClassStats are the counters build_class_index reports.
type ClassStats struct {
	FilesProcessed, FilesNoDecl, TopClasses, InnerClasses                 int
	Interfaces, Enums, AnnotationTypes, Records, Abstract, Packages, Mods int
}

// ClassIndex is the classes dict (Names in insertion order) and its stats.
type ClassIndex struct {
	Names   []string
	Classes map[string][]*ClassEntry
	Stats   ClassStats
	// ZKMSourceFound and ZKMModules describe the inventory the ZKM flags came
	// from (_meta zkm_source / zkm_modules).
	ZKMSourceFound bool
	ZKMModules     int
	Elapsed        time.Duration
}

// declaration is one parsed type declaration of a file.
type declaration struct {
	topLevel   bool // indent 0
	modifiers  []string
	kind, name string
	extends    optString
	implements []string
}

// parseJavaFile is parse_java_file: the declarations, line count and package
// of one source (none of them for an unreadable file). It no longer fails
// (core.Decl carries the match end); the error result keeps ScanClasses'
// contract unchanged.
func parseJavaFile(path string) ([]declaration, int, string, error) {
	text, err := pycompat.ReadTextReplace(path)
	if err != nil {
		return nil, 0, "", nil
	}
	lines := pycompat.Lines(text)
	pkg := ""
	for _, line := range lines[:min(30, len(lines))] {
		if name, ok := core.PackageName(pycompat.Strip(line)); ok {
			pkg = name
			break
		}
	}
	var decls []declaration
	inBlock := false
	for i, line := range lines {
		stripped := strings.TrimLeftFunc(line, pycompat.IsSpace)
		if inBlock {
			if strings.Contains(stripped, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(stripped, "/*") {
			if !strings.Contains(stripped, "*/") {
				inBlock = true
			}
			continue
		}
		if strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "*") {
			continue
		}
		d, ok := core.MatchDecl(line)
		if !ok {
			continue
		}
		ext, impl := parseExtendsImplements(javatext.DeclarationHeader(lines, i, d.End))
		decls = append(decls, declaration{
			topLevel:   d.Indent == "",
			modifiers:  pycompat.Fields(d.Modifiers),
			kind:       d.Kind,
			name:       d.Name,
			extends:    ext,
			implements: impl,
		})
	}
	return decls, len(lines), pkg, nil
}

// fileEntries is process_file: (class name, entry) pairs of one source. Every
// indent-0 declaration is a top-level type; the declarations after it, up to
// the next top-level one, are its inner types (the first declaration opens a
// group when none is at indent 0).
func fileEntries(f core.JavaFile, zkm bool) (names []string, entries []*ClassEntry, noDecl bool, err error) {
	decls, lineCount, pkg, err := parseJavaFile(f.Path)
	if err != nil || len(decls) == 0 {
		return nil, nil, true, err
	}
	type group struct {
		outer declaration
		inner []declaration
	}
	var groups []group
	for _, d := range decls {
		if d.topLevel || len(groups) == 0 {
			groups = append(groups, group{outer: d})
		} else {
			groups[len(groups)-1].inner = append(groups[len(groups)-1].inner, d)
		}
	}
	entry := func(d declaration, lines int, inner []string, outer string) *ClassEntry {
		return &ClassEntry{Package: pkg, Module: f.Sub, Kind: d.kind, Extends: d.extends,
			Implements: d.implements, Modifiers: d.modifiers, Lines: lines, Path: f.Rel,
			ZKM: zkm, InnerClasses: inner, Outer: outer}
	}
	for _, g := range groups {
		inner := make([]string, len(g.inner))
		for i, d := range g.inner {
			inner[i] = d.name
		}
		names = append(names, g.outer.name)
		entries = append(entries, entry(g.outer, lineCount, inner, ""))
		for _, d := range g.inner {
			names = append(names, d.name)
			entries = append(entries, entry(d, 0, []string{}, g.outer.name))
		}
	}
	return names, entries, false, nil
}

// ScanClasses is build_class_index(organized, zkm_modules): every vineflower
// .java file in corpus order, parsed by a few workers. It fails only on a
// parseJavaFile error (the first in corpus order).
func ScanClasses(organized string, zkmModules map[string]bool) (*ClassIndex, error) {
	t0 := time.Now()
	files := core.IterJava(organized)
	type result struct {
		names   []string
		entries []*ClassEntry
		noDecl  bool
		err     error
	}
	results := make([]result, len(files))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < min(4, runtime.GOMAXPROCS(0)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				r := &results[i]
				r.names, r.entries, r.noDecl, r.err = fileEntries(files[i], zkmModules[files[i].Sub])
			}
		}()
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()

	ci := &ClassIndex{Classes: map[string][]*ClassEntry{}}
	st := &ci.Stats
	packages, modules := map[string]bool{}, map[string]bool{}
	for i, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		st.FilesProcessed++
		modules[files[i].Sub] = true
		if r.noDecl {
			st.FilesNoDecl++
			continue
		}
		for j, e := range r.entries {
			if e.Package != "" {
				packages[e.Package] = true
			}
			if e.Outer != "" {
				st.InnerClasses++
			} else {
				st.TopClasses++
				switch e.Kind {
				case "interface":
					st.Interfaces++
				case "enum":
					st.Enums++
				case "@interface":
					st.AnnotationTypes++
				case "record":
					st.Records++
				}
				for _, m := range e.Modifiers {
					if m == "abstract" {
						st.Abstract++
						break
					}
				}
			}
			name := r.names[j]
			if _, ok := ci.Classes[name]; !ok {
				ci.Names = append(ci.Names, name)
			}
			ci.Classes[name] = append(ci.Classes[name], e)
		}
	}
	st.Packages, st.Mods = len(packages), len(modules)
	ci.Elapsed = time.Since(t0)
	return ci, nil
}

// LoadZKMModules is load_zkm_modules(base_dir): the submodules flagged ZKM in
// <baseDir>/indexes/module-inventory.json, and whether that file exists
// (without it every flag is false; the CLI warns). A file that is not the
// inventory's JSON shape is an error, where Python dies with a traceback.
func LoadZKMModules(baseDir string) (map[string]bool, bool, error) {
	path := IndexPath(baseDir, InventoryFile)
	if !pycompat.IsFile(path) {
		return map[string]bool{}, false, nil
	}
	text, err := pycompat.ReadText(path)
	if err != nil {
		return nil, true, fmt.Errorf("%s: %w", path, err)
	}
	doc, err := pyjson.Loads([]rune(text))
	if err != nil {
		return nil, true, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Kind != pyjson.Object {
		return nil, true, fmt.Errorf("%s: not a JSON object", path)
	}
	modules, ok := doc.Get("modules")
	if !ok {
		return map[string]bool{}, true, nil
	}
	if modules.Kind != pyjson.Object {
		return nil, true, fmt.Errorf("%s: modules is not a JSON object", path)
	}
	zkm := map[string]bool{}
	for _, name := range modules.Keys {
		info := modules.M[name]
		if info.Kind != pyjson.Object {
			return nil, true, fmt.Errorf("%s: modules[%q] is not a JSON object", path, name)
		}
		if v, _ := info.Get("zkm"); v.Truthy() {
			zkm[name] = true
		}
	}
	return zkm, true, nil
}

// Document is the class-index.json text: _meta (source = organized) and classes.
func (ci *ClassIndex) Document(organized string, at time.Time) string {
	total := 0
	for _, es := range ci.Classes {
		total += len(es)
	}
	zkmSource := pyjson.Value{}
	if ci.ZKMSourceFound {
		zkmSource = str(InventoryFile)
	}
	st := ci.Stats
	meta := stamped([]Field{
		{"description", str("Class index for Niagara N4 decompiled modules")},
		{"source", str(organized)},
		{"files_processed", integer(st.FilesProcessed)},
		{"modules", integer(st.Mods)},
		{"top_classes", integer(st.TopClasses)},
		{"inner_classes", integer(st.InnerClasses)},
		{"total_entries", integer(total)},
		{"unique_names", integer(len(ci.Names))},
		{"interfaces", integer(st.Interfaces)},
		{"enums", integer(st.Enums)},
		{"annotation_types", integer(st.AnnotationTypes)},
		{"records", integer(st.Records)},
		{"zkm_source", zkmSource},
		{"zkm_modules", integer(ci.ZKMModules)},
		{"abstract", integer(st.Abstract)},
		{"unique_packages", integer(st.Packages)},
	}, ci.Elapsed, at)
	w := &jsonWriter{o: Indented}
	w.object(0, 2, func(i int) string { return []string{"_meta", "classes"}[i] }, func(i, level int) {
		if i == 0 {
			w.fields(level, meta)
			return
		}
		w.object(level, len(ci.Names), func(j int) string { return ci.Names[j] }, func(j, l int) {
			es := ci.Classes[ci.Names[j]]
			a := pyjson.Value{Kind: pyjson.Array, A: make([]pyjson.Value, len(es))}
			for k, e := range es {
				a.A[k] = e.value()
			}
			w.value(a, l)
		})
	})
	return w.b.String()
}

func (e *ClassEntry) value() pyjson.Value {
	ext := pyjson.Value{}
	if e.Extends.valid {
		ext = str(e.Extends.s)
	}
	return object(
		Field{"package", str(e.Package)},
		Field{"module", str(e.Module)},
		Field{"kind", str(e.Kind)},
		Field{"extends", ext},
		Field{"implements", strList(e.Implements)},
		Field{"modifiers", strList(e.Modifiers)},
		Field{"lines", integer(e.Lines)},
		Field{"path", str(e.Path)},
		Field{"zkm", boolean(e.ZKM)},
		Field{"inner_classes", strList(e.InnerClasses)},
		Field{"outer_class", optStr(e.Outer)},
	)
}

// BuildClassIndex reads the ZKM flags from <baseDir>'s inventory, scans
// organized and writes <baseDir>/indexes/class-index.json
// (build_class_index.py main without the printed summary).
func BuildClassIndex(baseDir, organized string) (*ClassIndex, error) {
	t0 := time.Now()
	zkm, found, err := LoadZKMModules(baseDir)
	if err != nil {
		return nil, err
	}
	ci, err := ScanClasses(organized, zkm)
	if err != nil {
		return nil, err
	}
	ci.ZKMSourceFound, ci.ZKMModules = found, len(zkm)
	ci.Elapsed = time.Since(t0)
	if err := writeIndex(baseDir, ClassIndexFile, ci.Document(organized, now())); err != nil {
		return nil, fmt.Errorf("writing %s: %w", ClassIndexFile, err)
	}
	return ci, nil
}
