package typecatalog

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// A catalog file is read into the values Python's json.load builds, so show prints str() of
// whatever a hand-edited catalog holds (flags "abc", 1026.0, a list, null...) like the Python
// tool; only what Python cannot print (a KeyError or TypeError traceback there) is reported as
// a malformed catalog.

// pyDict is a JSON object as a Python dict: insertion order, a repeated key keeps its first
// position and its last value.
type pyDict struct {
	keys []string
	m    map[string]any
}

func (d *pyDict) set(k string, v any) {
	if _, ok := d.m[k]; !ok {
		d.keys = append(d.keys, k)
	}
	d.m[k] = v
}

func (d *pyDict) get(k string) (any, bool) {
	v, ok := d.m[k]
	return v, ok
}

// String is repr(dict), which str() of a dict also is.
func (d *pyDict) String() string {
	parts := make([]string, len(d.keys))
	for i, k := range d.keys {
		parts[i] = pycompat.StrRepr(k) + ": " + pycompat.Repr(d.m[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// decodeValue reads one JSON value as json.load does: int (unbounded) for a number without
// '.', 'e' or 'E', float otherwise.
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch x := tok.(type) {
	case json.Delim:
		if x == '[' {
			list := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, v)
			}
			_, err := dec.Token()
			return list, err
		}
		d := &pyDict{m: map[string]any{}}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			d.set(k.(string), v)
		}
		_, err := dec.Token()
		return d, err
	case json.Number:
		if strings.ContainsAny(string(x), ".eE") {
			return pycompat.ParseFloat(string(x))
		}
		n, ok := new(big.Int).SetString(string(x), 10)
		if !ok {
			return nil, fmt.Errorf("bad number %s", x)
		}
		return n, nil
	}
	return tok, nil // string, bool, nil
}

// malformed is an entry the Python tool would crash on.
type malformed struct{ msg string }

func (m *malformed) Error() string { return m.msg }

func badf(format string, a ...any) error { return &malformed{fmt.Sprintf(format, a...)} }

// sourceOf is t.get("source", "") of a loaded entry for module scoping (`source or ""`).
func sourceOf(key string, v any) (string, error) {
	d, ok := v.(*pyDict)
	if !ok {
		return "", badf("%s: not an object", key)
	}
	s, ok := d.get("source")
	if !ok || !truthy(s) {
		return "", nil
	}
	if str, isStr := s.(string); isStr {
		return str, nil
	}
	return "", badf("%s: source is not a string", key)
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case *big.Int:
		return x.Sign() != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case *pyDict:
		return len(x.keys) > 0
	}
	return true
}

// entryView is what show prints of one entry: the header fields and each declaration's
// name, flags, flagLetters, default and facets as str().
type entryView struct {
	extends, source string
	groups          [3][]declView
}

var groupNames = [3]string{"properties", "actions", "topics"}

// viewLoaded is the entry view of a loaded catalog value, or the malformed-entry error.
func viewLoaded(key string, v any) (entryView, error) {
	var ev entryView
	d, ok := v.(*pyDict)
	if !ok {
		return ev, badf("%s: not an object", key)
	}
	ext, ok := d.get("extends")
	if !ok {
		return ev, badf(`%s: no "extends"`, key)
	}
	ev.extends = pycompat.Str(ext)
	if s, ok := d.get("source"); ok {
		ev.source = pycompat.Str(s)
	}
	for g, name := range groupNames {
		items, ok := d.get(name)
		if !ok {
			return ev, badf(`%s: no %q`, key, name)
		}
		list, isList := items.([]any)
		if !isList {
			if truthy(items) || !iterable(items) {
				return ev, badf("%s: %s is not a list", key, name)
			}
		}
		for i, it := range list {
			decl, ok := it.(*pyDict)
			if !ok {
				return ev, badf("%s: %s[%d] is not an object", key, name, i)
			}
			var f [5]string
			for j, field := range []string{"name", "flags", "flagLetters", "default", "facets"} {
				fv, ok := decl.get(field)
				if !ok {
					return ev, badf(`%s: %s[%d] has no %q`, key, name, i, field)
				}
				f[j] = pycompat.Str(fv)
			}
			ev.groups[g] = append(ev.groups[g], declView{f[0], f[1], f[2], f[3], f[4]})
		}
	}
	return ev, nil
}

// iterable: an empty str or dict iterates nothing in Python; numbers, bools and None raise.
func iterable(v any) bool {
	switch v.(type) {
	case string, *pyDict, []any:
		return true
	}
	return false
}
