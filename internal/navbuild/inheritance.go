package navbuild

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The inheritance index (build_inheritance.py): parent -> children,
// interface -> implementors and class -> extends chain over the top-level
// entries of the class index, indexes/inheritance.json.

// InheritanceFile is the index the inheritance builder writes.
const InheritanceFile = "inheritance.json"

// Inheritance is what build_inheritance computes.
type Inheritance struct {
	ParentToChildren, InterfaceToImplementors *orderedLists
	// ChainNames are the classes with a chain, in extends_map order.
	ChainNames                                                []string
	Chains                                                    map[string][]string
	TopLevel, WithExtends, WithImplements, MaxDepth, Circular int
	Elapsed                                                   time.Duration
}

// ComputeInheritance is build_inheritance's two passes over the top-level entries.
func ComputeInheritance(entries []classIndexEntry) *Inheritance {
	in := &Inheritance{ParentToChildren: newOrderedLists(), InterfaceToImplementors: newOrderedLists(),
		Chains: map[string][]string{}}
	extendsOf := map[string]string{}
	var extendsOrder []string
	for _, e := range entries {
		if e.Outer {
			continue
		}
		in.TopLevel++
		if e.Extends != "" {
			in.WithExtends++
			in.ParentToChildren.add(e.Extends, e.Name)
			if _, ok := extendsOf[e.Name]; !ok {
				extendsOf[e.Name] = e.Extends
				extendsOrder = append(extendsOrder, e.Name)
			}
		}
		if len(e.Implmnts) > 0 {
			in.WithImplements++
			for _, iface := range e.Implmnts {
				in.InterfaceToImplementors.add(iface, e.Name)
			}
		}
	}
	in.ParentToChildren.sortValues()
	in.InterfaceToImplementors.sortValues()
	for _, name := range extendsOrder {
		var chain []string
		seen := map[string]bool{name: true}
		for cur := extendsOf[name]; cur != ""; cur = extendsOf[cur] {
			if seen[cur] {
				in.Circular++
				break
			}
			chain = append(chain, cur)
			seen[cur] = true
		}
		if len(chain) > 0 {
			in.ChainNames = append(in.ChainNames, name)
			in.Chains[name] = chain
			in.MaxDepth = max(in.MaxDepth, len(chain))
		}
	}
	return in
}

// Document is the inheritance.json text (compact).
func (in *Inheritance) Document(at time.Time) string {
	depthCount := map[int]int{}
	sumDepth := 0
	for _, n := range in.ChainNames {
		depthCount[len(in.Chains[n])]++
		sumDepth += len(in.Chains[n])
	}
	depths := make([]int, 0, len(depthCount))
	for d := range depthCount {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	dist := pyjson.NewObject()
	for _, d := range depths {
		dist.Set(strconv.Itoa(d), integer(depthCount[d]))
	}
	children, impls := in.ParentToChildren.total(), in.InterfaceToImplementors.total()
	meta := stamped([]Field{
		{"description", str("Inheritance index for Niagara N4 decompiled modules")},
		{"source_index", str(ClassIndexFile)},
		{"top_level_processed", integer(in.TopLevel)},
		{"with_extends", integer(in.WithExtends)},
		{"with_implements", integer(in.WithImplements)},
		{"unique_parents", integer(len(in.ParentToChildren.keys))},
		{"unique_interfaces_implemented", integer(len(in.InterfaceToImplementors.keys))},
		{"classes_with_chains", integer(len(in.ChainNames))},
		{"max_chain_depth", integer(in.MaxDepth)},
		{"circular_references", integer(in.Circular)},
		{"total_children_entries", integer(children)},
		{"total_implementor_entries", integer(impls)},
		{"avg_children_per_parent", float(round1(float64(children) / float64(max(len(in.ParentToChildren.keys), 1))))},
		{"avg_chain_depth", float(round1(float64(sumDepth) / float64(max(len(in.ChainNames), 1))))},
		{"depth_distribution", dist},
	}, in.Elapsed, at)
	w := &jsonWriter{o: Compact}
	keys := []string{"_meta", "parent_to_children", "interface_to_implementors", "class_to_chain"}
	w.object(0, len(keys), func(i int) string { return keys[i] }, func(i, level int) {
		switch i {
		case 0:
			w.fields(level, meta)
		case 1:
			w.lists(level, in.ParentToChildren)
		case 2:
			w.lists(level, in.InterfaceToImplementors)
		default:
			w.object(level, len(in.ChainNames), func(j int) string { return in.ChainNames[j] },
				func(j, l int) { w.value(strList(in.Chains[in.ChainNames[j]]), l) })
		}
	})
	return w.b.String()
}

// BuildInheritance reads <baseDir>'s class index and writes
// <baseDir>/indexes/inheritance.json (build_inheritance without the summary).
func BuildInheritance(baseDir string) (*Inheritance, error) {
	t0 := time.Now()
	entries, err := readClassIndex(baseDir)
	if err != nil {
		return nil, err
	}
	in := ComputeInheritance(entries)
	in.Elapsed = time.Since(t0)
	if err := writeIndex(baseDir, InheritanceFile, in.Document(now())); err != nil {
		return nil, fmt.Errorf("writing %s: %w", InheritanceFile, err)
	}
	return in, nil
}
