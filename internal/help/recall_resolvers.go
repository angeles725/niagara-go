package help

// The resolvers the recall oracle calls for the lookups of commands the Go
// port retires (hierarchy, implementors, enum, module), kept as resolvers
// only: hierarchy._key_maps (resolve_inheritance_key, built once per run),
// implementor_matches, enum_search.resolve_enum, module_class_matches, and
// the class -> keys map behind slots_search.class_slot_entries.

import (
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// inheritanceKeys is hierarchy._key_maps: the lookups resolve_inheritance_key
// tries after an exact key, built once.
type inheritanceKeys struct {
	inh                    *index.Object
	stripped, qual, lowerK map[string]string
}

func newInheritanceKeys(inh *index.Object) *inheritanceKeys {
	k := &inheritanceKeys{inh, map[string]string{}, map[string]string{}, map[string]string{}}
	setDefault := func(m map[string]string, key, v string) {
		if _, ok := m[key]; !ok {
			m[key] = v
		}
	}
	for _, key := range inh.Keys() {
		s := stripGenerics(key)
		setDefault(k.stripped, s, key)
		setDefault(k.lowerK, lowerStr(s), key)
		if pv := get(inh.Object(key), "package", nil); index.Truthy(pv) {
			pkg, simple := pyStr(pv), s
			if strings.HasPrefix(s, pkg+".") {
				simple = s[len(pkg)+1:]
			}
			setDefault(k.qual, pkg+"."+simple, key)
		}
	}
	return k
}

// resolve is resolve_inheritance_key ("" when nothing matches).
func (k *inheritanceKeys) resolve(name string) string {
	if k.inh.Has(name) {
		return name
	}
	if v := k.stripped[name]; v != "" {
		return v
	}
	if v := k.qual[name]; v != "" {
		return v
	}
	return k.lowerK[lowerStr(name)]
}

// isEnumType is enum_search._is_enum_type: the class extends an enum root.
func isEnumType(inh *index.Object, name string) bool {
	seen := map[string]bool{}
	for cur := name; cur != "" && !seen[cur]; {
		seen[cur] = true
		if recallEnumRoots[cur] {
			return true
		}
		cur = getStr(getObj(inh, cur), "extends", "")
	}
	return false
}

// resolveEnum is enum_search.resolve_enum reduced to whether it finds the
// enum: through source-index.json, else through inheritance.json.
func resolveEnum(src, inh *index.Object, keys *inheritanceKeys, name string) bool {
	key, ok := name, src.Has(name)
	if !ok {
		want := lowerStr(name)
		for _, k := range src.Keys() {
			if lowerStr(k) == want {
				key, ok = k, true
				break
			}
		}
	}
	if ok && isEnumType(inh, strings.SplitN(key, " (", 2)[0]) {
		return true
	}
	if ikey := keys.resolve(name); ikey != "" && isEnumType(inh, ikey) {
		return true
	}
	return ok
}

// slotKeys maps a lower-case class name to the slots-index keys that
// class_slot_entries matches for it (the key itself and "Name (module)").
func slotKeys(si *index.Object) map[string][]string {
	m := map[string][]string{}
	for _, k := range si.Keys() {
		low := lowerStr(k)
		m[low] = append(m[low], k)
		for i := 0; ; {
			j := strings.Index(low[i:], " (")
			if j < 0 {
				break
			}
			if prefix := low[:i+j]; prefix != low {
				m[prefix] = append(m[prefix], k)
			}
			i += j + 1
		}
	}
	return m
}

var htmlGenerics = regexp.MustCompile(`&lt;.*?&gt;`)

// stripGenerics removes HTML-encoded generics such as &lt;E&gt; from a key.
func stripGenerics(name string) string { return htmlGenerics.ReplaceAllString(name, "") }

// bareType is _bare: lower-case type name without generics, "$" as ".".
func bareType(t string) string {
	s := stripGenerics(t)
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[:i]
	}
	return lowerStr(strings.ReplaceAll(pycompat.Strip(s), "$", "."))
}

type implementor struct{ name, pkg string }

// implementorMatches is hierarchy.implementor_matches: the classes (key
// without generics, package) implementing iface. A simple name matches every
// interface with that simple name, a qualified one only that interface.
func implementorMatches(inh *index.Object, iface string) []implementor {
	want := bareType(iface)
	wantSimple := want[strings.LastIndex(want, ".")+1:]
	qualified := strings.Contains(want, ".")
	var out []implementor
	for _, key := range inh.Keys() {
		entry := inh.Object(key)
		for _, impl := range strList(entry, "implements") {
			got := bareType(impl)
			var same bool
			if qualified && strings.Contains(got, ".") {
				same = got == want
			} else {
				same = got[strings.LastIndex(got, ".")+1:] == wantSimple
			}
			if same {
				pkg := getStr(entry, "package", "")
				name := stripGenerics(key)
				if pkg != "" && strings.HasPrefix(name, pkg+".") {
					name = name[len(pkg)+1:] // FQN key of a collision
				}
				out = append(out, implementor{name, pkg})
				break
			}
		}
	}
	return out
}

// moduleSuffix is r"-(rt|ux|wb|se|doc)$"; Python's $ also matches before a
// final newline, which the optional group keeps.
var moduleSuffix = regexp.MustCompile(`-(?:rt|ux|wb|se|doc)(\n?)$`)

// moduleKey is _module_key: "alarm-rt" -> "alarm", "docAlarms" -> "alarm".
func moduleKey(name string) string {
	clean := moduleSuffix.ReplaceAllString(lowerStr(name), "$1")
	clean = strings.TrimPrefix(clean, "doc")
	return strings.TrimRight(clean, "s")
}

type namedEntry struct {
	name string
	data *index.Object
}

// moduleClassMatches is module_class_matches: every class variant whose
// module equals the name or contains the key, or whose name or package
// contains the key.
func moduleClassMatches(idx *index.Object, module string) []namedEntry {
	clean, exact := moduleKey(module), lowerStr(module)
	var out []namedEntry
	for _, name := range idx.Keys() {
		entry := idx.Object(name)
		variants := append([]*index.Object{entry}, objList(entry, "others")...)
		for _, d := range variants {
			mod, pkg := lowerStr(getStr(d, "module", "")), lowerStr(getStr(d, "package", ""))
			if mod == exact || (clean != "" && strings.Contains(mod, clean)) ||
				strings.Contains(lowerStr(name), clean) || strings.Contains(pkg, clean) {
				out = append(out, namedEntry{name, d})
			}
		}
	}
	return out
}
