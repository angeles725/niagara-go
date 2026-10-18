package navbuild

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
)

// The call graph index (build_callgraph_index.py): "Class.method" -> the
// sorted "Target.method" calls found in that method's zone (from its
// declaration line to the next declaration of the file), over the top-level
// sources of the class index, indexes/callgraph-index.json (compact JSON).

// CallgraphFile is the index the call graph builder writes.
const CallgraphFile = "callgraph-index.json"

// Callgraph is what build_callgraph_index computes.
type Callgraph struct {
	FilesParsed, FilesFailed int
	// Calls maps each caller key to its callees.
	Calls map[string]map[string]bool
	// InheritanceFound is false when indexes/inheritance.json is absent
	// (super.method() then resolves to the caller's own class).
	InheritanceFound bool
	Elapsed          time.Duration
}

// readParentMap is the parent_map of build_callgraph_index: class -> the
// first name of its class_to_chain (its direct parent), from
// <baseDir>/indexes/inheritance.json when that file exists.
func readParentMap(baseDir string) (map[string]string, bool, error) {
	path := IndexPath(baseDir, InheritanceFile)
	parents := map[string]string{}
	if !isFile(path) {
		return parents, false, nil
	}
	v, err := index.LoadPython(path)
	if err != nil {
		return nil, true, err
	}
	doc, ok := v.(*index.Object)
	if !ok {
		return nil, true, fmt.Errorf("%s: not a JSON object", path)
	}
	chains, ok := doc.Get("class_to_chain")
	if !ok {
		return parents, true, nil
	}
	obj, ok := chains.(*index.Object)
	if !ok {
		return nil, true, fmt.Errorf("%s: class_to_chain is not a JSON object", path)
	}
	for i, cls := range obj.Keys() {
		chain, ok := obj.Values()[i].([]any)
		if !ok {
			if index.Truthy(obj.Values()[i]) {
				return nil, true, fmt.Errorf("%s: class_to_chain[%q] is not a list", path, cls)
			}
			continue
		}
		if len(chain) == 0 {
			continue
		}
		parent, ok := chain[0].(string)
		if !ok {
			return nil, true, fmt.Errorf("%s: class_to_chain[%q] starts with a non-string", path, cls)
		}
		parents[cls] = parent
	}
	return parents, true, nil
}

// simpleParent is the parent as a simple name ("org.x.Base<T>" -> "Base"),
// "" when there is none.
func simpleParent(parents map[string]string, class string) string {
	p := parents[class]
	if i := strings.IndexByte(p, '<'); i >= 0 {
		p = p[:i]
	}
	return p[strings.LastIndexByte(p, '.')+1:]
}

// fileCalls is parse_calls_from_file: the callees of each method key of one
// source, attributed by zone.
func fileCalls(content, fileClass string, parents map[string]string) map[string]map[string]bool {
	methods := parseMethods(content, fileClass) // already in line order
	if len(methods) == 0 {
		return nil
	}
	lines := strings.Split(content, "\n")
	calls := map[string]map[string]bool{}
	for i, m := range methods {
		end := len(lines)
		if i+1 < len(methods) {
			end = methods[i+1].Line - 1
		}
		var zone []string
		if start := m.Line - 1; start < end {
			zone = lines[start:end]
		}
		callees := zoneCalls(zone, m.Class, simpleParent(parents, m.Class))
		if len(callees) == 0 {
			continue
		}
		key := m.Class + "." + m.IndexedName
		if set, ok := calls[key]; ok {
			for c := range callees {
				set[c] = true
			}
		} else {
			calls[key] = callees
		}
	}
	return calls
}

type parsedCalls struct {
	ok    bool
	calls map[string]map[string]bool
}

// ComputeCallgraph parses the top-level sources of entries under organized.
func ComputeCallgraph(entries []classIndexEntry, organized string, parents map[string]string) (*Callgraph, error) {
	files := sourceFiles(entries, organized)
	cg := &Callgraph{Calls: map[string]map[string]bool{}}
	err := inOrder(len(files), func(i int) parsedCalls {
		content, ok := readSource(files[i].full)
		if !ok {
			return parsedCalls{}
		}
		return parsedCalls{true, fileCalls(content, files[i].className, parents)}
	}, func(_ int, p parsedCalls) error {
		if !p.ok {
			cg.FilesFailed++
			return nil
		}
		cg.FilesParsed++
		for key, callees := range p.calls {
			if set, ok := cg.Calls[key]; ok {
				for c := range callees {
					set[c] = true
				}
			} else {
				cg.Calls[key] = callees
			}
		}
		return nil
	})
	return cg, err
}

// write streams callgraph-index.json: keys and callees sorted.
func (cg *Callgraph) write(dst *bufio.Writer, at time.Time) error {
	keys := make([]string, 0, len(cg.Calls))
	for k := range cg.Calls {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	sorted := make(map[string][]string, len(keys))
	unique := map[string]bool{}
	edges := 0
	for _, k := range keys {
		cs := make([]string, 0, len(cg.Calls[k]))
		for c := range cg.Calls[k] {
			cs = append(cs, c)
			unique[c] = true
		}
		slices.Sort(cs)
		sorted[k] = cs
		edges += len(cs)
	}
	meta := stamped([]Field{
		{"description", str("Call graph index for Niagara N4 decompiled modules")},
		{"source_indexes", strList([]string{ClassIndexFile, InheritanceFile})},
		{"files_parsed", integer(cg.FilesParsed)},
		{"files_failed", integer(cg.FilesFailed)},
		{"total_caller_methods", integer(len(keys))},
		{"total_unique_callees", integer(len(unique))},
		{"total_edges", integer(edges)},
		{"avg_callees_per_caller", float(round1(float64(edges) / float64(max(len(keys), 1))))},
	}, cg.Elapsed, at)
	w := &jsonWriter{o: Compact, dst: dst}
	top := []string{"_meta", "calls"}
	w.object(0, len(top), func(i int) string { return top[i] }, func(i, level int) {
		if i == 0 {
			w.fields(level, meta)
			return
		}
		w.object(level, len(keys), func(j int) string { return keys[j] }, func(j, _ int) {
			w.put(encodeStrings(sorted[keys[j]]))
		})
	})
	return nil
}

// BuildCallgraph reads <baseDir>'s class and inheritance indexes and the
// sources under organized and writes <baseDir>/indexes/callgraph-index.json
// (build_callgraph_index without the printed summary).
func BuildCallgraph(baseDir, organized string) (*Callgraph, error) {
	t0 := time.Now()
	entries, err := readClassIndex(baseDir)
	if err != nil {
		return nil, err
	}
	parents, found, err := readParentMap(baseDir)
	if err != nil {
		return nil, err
	}
	cg, err := ComputeCallgraph(entries, organized, parents)
	if err != nil {
		return nil, err
	}
	cg.InheritanceFound = found
	cg.Elapsed = time.Since(t0)
	if err := writeIndexStream(baseDir, CallgraphFile, func(w *bufio.Writer) error { return cg.write(w, now()) }); err != nil {
		return nil, fmt.Errorf("writing %s: %w", CallgraphFile, err)
	}
	return cg, nil
}
