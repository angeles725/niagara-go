package help

// The resolvers the recall oracle calls once per item, built once per run:
// hierarchy._key_maps (resolve_inheritance_key), enum_search.resolve_enum
// and the class -> keys map behind slots_search.class_slot_entries.

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
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
		if pv := get(inh.Object(key), "package", nil); truthy(pv) {
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
