package navbuild

import (
	"bufio"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The method index (build_method_index.py): method name -> its declarations
// and class -> its method names, over the top-level sources of the class
// index, indexes/method-index.json (compact JSON).

// MethodFile is the index the method builder writes.
const MethodFile = "method-index.json"

// MethodIndex is what build_method_index computes.
type MethodIndex struct {
	FilesParsed, FilesFailed, TotalConstructors int
	// Names are the method names in first-seen order; Entries their
	// declarations, each already encoded as its compact JSON object.
	Names   []string
	Entries map[string][]string
	// Classes are the classes with methods in first-seen order; Members their
	// method names.
	Classes     []string
	Members     map[string]map[string]bool
	MaxPerClass int
	MaxClass    string
	Elapsed     time.Duration
	classOrder  map[string]int
	touched     []string
}

// sourceFile is one top-level file a member builder parses.
type sourceFile struct {
	full, className string
	module          pyjson.Value // e.get("module", ""): a str, or None
}

// sourceFiles is the builders' top_files with each path joined to organized.
func sourceFiles(entries []classIndexEntry, organized string) []sourceFile {
	var out []sourceFile
	for _, e := range topLevelFiles(entries) {
		mod := str(e.Module)
		if e.ModuleNull {
			mod = pyjson.Value{}
		}
		out = append(out, sourceFile{sourcePath(organized, e.Path), e.Name, mod})
	}
	return out
}

// readSource is the builders' isfile check and open(errors="replace").read();
// ok is false for a file that counts as failed.
func readSource(path string) (string, bool) {
	if !pycompat.IsFile(path) {
		return "", false
	}
	content, err := pycompat.ReadTextReplace(path)
	return content, err == nil
}

// methodRecord is one declaration ready to merge: its indexed name, class and
// encoded entry (strings that do not keep the file content alive).
type methodRecord struct{ name, class, entry string }

type parsedMethods struct {
	ok      bool
	records []methodRecord
}

func parseMethodFile(f sourceFile) parsedMethods {
	content, ok := readSource(f.full)
	if !ok {
		return parsedMethods{}
	}
	module := pyjson.Dumps(f.module, Compact)
	ms := parseMethods(content, f.className)
	out := parsedMethods{ok: true, records: make([]methodRecord, len(ms))}
	for i, m := range ms {
		out.records[i] = methodRecord{strings.Clone(m.IndexedName), strings.Clone(m.Class), methodEntry(m, module)}
	}
	return out
}

// methodEntry is the compact JSON of a methods-map entry, keys in Python's order.
func methodEntry(m javaMethod, module string) string {
	var b strings.Builder
	b.WriteString(`{"class":` + pyjson.EncodeString(m.Class, false))
	b.WriteString(`,"module":` + module)
	b.WriteString(`,"signature":` + pyjson.EncodeString(m.Signature, false))
	b.WriteString(`,"line":` + strconv.Itoa(m.Line))
	b.WriteString(`,"modifiers":` + encodeStrings(m.Modifiers))
	b.WriteString(`,"return_type":` + pyjson.EncodeString(m.ReturnType, false))
	b.WriteString(`,"params":` + encodeStrings(m.Params) + "}")
	return b.String()
}

func encodeStrings(ss []string) string {
	enc := make([]string, len(ss))
	for i, s := range ss {
		enc[i] = pyjson.EncodeString(s, false)
	}
	return "[" + strings.Join(enc, ",") + "]"
}

func newMethodIndex() *MethodIndex {
	return &MethodIndex{Entries: map[string][]string{}, Members: map[string]map[string]bool{},
		classOrder: map[string]int{}}
}

// add merges one file's declarations, then updates the largest class like
// Python's rescan of every class after each file: the first class, in
// first-seen order, whose method count is strictly greater than the running
// maximum. Only the classes this file touched can have grown past it, so
// only they are visited, in that same order.
func (mi *MethodIndex) add(records []methodRecord) {
	mi.touched = mi.touched[:0]
	for _, r := range records {
		if _, ok := mi.Entries[r.name]; !ok {
			mi.Names = append(mi.Names, r.name)
		}
		mi.Entries[r.name] = append(mi.Entries[r.name], r.entry)
		set, ok := mi.Members[r.class]
		if !ok {
			set = map[string]bool{}
			mi.Members[r.class] = set
			mi.classOrder[r.class] = len(mi.Classes)
			mi.Classes = append(mi.Classes, r.class)
		}
		set[r.name] = true
		mi.touched = append(mi.touched, r.class)
		if r.name == "<init>" {
			mi.TotalConstructors++
		}
	}
	slices.SortFunc(mi.touched, func(a, b string) int { return mi.classOrder[a] - mi.classOrder[b] })
	for _, c := range slices.Compact(mi.touched) {
		if n := len(mi.Members[c]); n > mi.MaxPerClass {
			mi.MaxPerClass, mi.MaxClass = n, c
		}
	}
}

// ComputeMethods parses the top-level sources of entries under organized.
func ComputeMethods(entries []classIndexEntry, organized string) (*MethodIndex, error) {
	files := sourceFiles(entries, organized)
	mi := newMethodIndex()
	err := inOrder(len(files), func(i int) parsedMethods { return parseMethodFile(files[i]) },
		func(_ int, p parsedMethods) error {
			if !p.ok {
				mi.FilesFailed++
				return nil
			}
			mi.FilesParsed++
			mi.add(p.records)
			return nil
		})
	return mi, err
}

// definitionStats are the per-name and per-class averages and the largest
// name of a member index; an average over nothing is the int 0, as Python's
// `else 0`.
func definitionStats(names []string, defs func(string) int, classes []string, members func(string) int) (total, maxDefs int, avgDefs, avgPerClass pyjson.Value) {
	avgDefs, avgPerClass = integer(0), integer(0)
	for _, n := range names {
		total += defs(n)
		maxDefs = max(maxDefs, defs(n))
	}
	if len(names) > 0 {
		avgDefs = float(round1(float64(total) / float64(len(names))))
	}
	sum := 0
	for _, c := range classes {
		sum += members(c)
	}
	if len(classes) > 0 {
		avgPerClass = float(round1(float64(sum) / float64(len(classes))))
	}
	return total, maxDefs, avgDefs, avgPerClass
}

// write streams method-index.json.
func (mi *MethodIndex) write(dst *bufio.Writer, at time.Time) error {
	total, maxDefs, avgDefs, avgPerClass := definitionStats(mi.Names, func(n string) int { return len(mi.Entries[n]) },
		mi.Classes, func(c string) int { return len(mi.Members[c]) })
	meta := stamped([]Field{
		{"description", str("Method index for Niagara N4 decompiled modules")},
		{"source_index", str(ClassIndexFile)},
		{"files_parsed", integer(mi.FilesParsed)},
		{"files_failed", integer(mi.FilesFailed)},
		{"total_method_definitions", integer(total)},
		{"unique_method_names", integer(len(mi.Names))},
		{"total_constructors", integer(mi.TotalConstructors)},
		{"classes_with_methods", integer(len(mi.Classes))},
		{"avg_methods_per_class", avgPerClass},
		{"max_methods_per_class", integer(mi.MaxPerClass)},
		{"max_methods_class", str(mi.MaxClass)},
		{"max_definitions_per_name", integer(maxDefs)},
		{"avg_definitions_per_name", avgDefs},
	}, mi.Elapsed, at)
	writeMemberIndex(dst, meta, "methods", mi.Names, mi.Entries, "class_methods", mi.Classes, mi.Members)
	return nil
}

// writeMemberIndex writes {"_meta": meta, byName: {name: [entries]},
// byClass: {class: sorted names}} in the compact layout.
func writeMemberIndex(dst *bufio.Writer, meta []Field, byName string, names []string, entries map[string][]string,
	byClass string, classes []string, members map[string]map[string]bool) {
	w := &jsonWriter{o: Compact, dst: dst}
	keys := []string{"_meta", byName, byClass}
	w.object(0, len(keys), func(i int) string { return keys[i] }, func(i, level int) {
		switch i {
		case 0:
			w.fields(level, meta)
		case 1:
			w.object(level, len(names), func(j int) string { return names[j] }, func(j, _ int) {
				w.put("[" + strings.Join(entries[names[j]], ",") + "]")
			})
		default:
			w.object(level, len(classes), func(j int) string { return classes[j] }, func(j, _ int) {
				sorted := make([]string, 0, len(members[classes[j]]))
				for n := range members[classes[j]] {
					sorted = append(sorted, n)
				}
				slices.Sort(sorted)
				w.put(encodeStrings(sorted))
			})
		}
	})
}

// BuildMethodIndex reads <baseDir>'s class index and the sources under
// organized and writes <baseDir>/indexes/method-index.json
// (build_method_index without the printed summary).
func BuildMethodIndex(baseDir, organized string) (*MethodIndex, error) {
	t0 := time.Now()
	entries, err := readClassIndex(baseDir)
	if err != nil {
		return nil, err
	}
	mi, err := ComputeMethods(entries, organized)
	if err != nil {
		return nil, err
	}
	mi.Elapsed = time.Since(t0)
	if err := writeIndexStream(baseDir, MethodFile, func(w *bufio.Writer) error { return mi.write(w, now()) }); err != nil {
		return nil, fmt.Errorf("writing %s: %w", MethodFile, err)
	}
	return mi, nil
}
