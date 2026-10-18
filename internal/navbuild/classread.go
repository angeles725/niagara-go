package navbuild

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/angeles725/niagara-go/internal/index"
)

// What the builders after the class index share: its entries read back, and
// the str -> list maps they fill.

// classIndexEntry is one class-index entry as the later builders read it.
type classIndexEntry struct {
	Name     string // its class name (the classes key)
	Outer    bool   // truthy outer_class: an inner type
	Extends  string // "" for a falsy extends
	Implmnts []string
	Package  string
	Module   string
	Path     string
}

// readClassIndex is build.read_json(<baseDir>/indexes/class-index.json)["classes"]
// flattened in dict order (a missing "classes" key is empty, as Python's
// ci_data.get("classes", {})). A missing index is an error naming it; values of an
// unexpected type are errors too (Python would die or index them oddly).
func readClassIndex(baseDir string) ([]classIndexEntry, error) {
	path := IndexPath(baseDir, ClassIndexFile)
	if !isFile(path) {
		return nil, fmt.Errorf("%s not found at %s. Run ./reindex.sh (it builds the indexes in dependency order)", ClassIndexFile, path)
	}
	v, err := index.LoadPython(path)
	if err != nil {
		return nil, err
	}
	doc, ok := v.(*index.Object)
	if !ok {
		return nil, fmt.Errorf("%s: not a JSON object", path)
	}
	classes := index.NewObject()
	if c, ok := doc.Get("classes"); ok {
		if classes, ok = c.(*index.Object); !ok {
			return nil, fmt.Errorf("%s: classes is not a JSON object", path)
		}
	}
	var out []classIndexEntry
	for i, name := range classes.Keys() {
		entries, ok := classes.Values()[i].([]any)
		if !ok {
			return nil, fmt.Errorf("%s: classes[%q] is not a list", path, name)
		}
		for _, ev := range entries {
			e, ok := ev.(*index.Object)
			if !ok {
				return nil, fmt.Errorf("%s: an entry of classes[%q] is not an object", path, name)
			}
			ce := classIndexEntry{Name: name}
			outer, _ := e.Get("outer_class")
			ce.Outer = truthy(outer)
			var errs [4]error
			ce.Extends, errs[0] = optField(e, "extends")
			ce.Package, errs[1] = optField(e, "package")
			ce.Module, errs[2] = optField(e, "module")
			ce.Path, errs[3] = optField(e, "path")
			for _, err := range errs {
				if err != nil {
					return nil, fmt.Errorf("%s: classes[%q]: %w", path, name, err)
				}
			}
			if impl, ok := e.Get("implements"); ok && impl != nil {
				list, ok := impl.([]any)
				if !ok {
					return nil, fmt.Errorf("%s: classes[%q]: implements is not a list", path, name)
				}
				for _, x := range list {
					s, ok := x.(string)
					if !ok {
						return nil, fmt.Errorf("%s: classes[%q]: implements holds a non-string", path, name)
					}
					ce.Implmnts = append(ce.Implmnts, s)
				}
			}
			out = append(out, ce)
		}
	}
	return out, nil
}

// optField is a string member, "" when absent or null; any other type is an error.
func optField(e *index.Object, key string) (string, error) {
	v, ok := e.Get(key)
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s is not a string", key)
	}
	return s, nil
}

// truthy is bool(v) for a decoded JSON value.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case *index.Object:
		return t.Len() > 0
	default: // json.Number
		f, err := strconv.ParseFloat(fmt.Sprint(t), 64)
		return err != nil || f != 0
	}
}

// orderedLists is a dict of str -> list in insertion order, each list without
// repeats (the `if x not in d[k]` appends of the builders).
type orderedLists struct {
	keys []string
	vals map[string][]string
	seen map[string]map[string]bool
}

func newOrderedLists() *orderedLists {
	return &orderedLists{vals: map[string][]string{}, seen: map[string]map[string]bool{}}
}

func (o *orderedLists) add(k, v string) {
	s, ok := o.seen[k]
	if !ok {
		s = map[string]bool{}
		o.seen[k] = s
		o.keys = append(o.keys, k)
	}
	if !s[v] {
		s[v] = true
		o.vals[k] = append(o.vals[k], v)
	}
}

func (o *orderedLists) sortValues() {
	for _, k := range o.keys {
		sort.Strings(o.vals[k])
	}
}

func (o *orderedLists) total() int {
	n := 0
	for _, k := range o.keys {
		n += len(o.vals[k])
	}
	return n
}

// lists writes o as an object of string lists at depth level.
func (w *jsonWriter) lists(level int, o *orderedLists) {
	w.object(level, len(o.keys), func(i int) string { return o.keys[i] },
		func(i, l int) { w.value(strList(o.vals[o.keys[i]]), l) })
}
