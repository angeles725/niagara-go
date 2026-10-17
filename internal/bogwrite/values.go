package bogwrite

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// A plan is json.load's value model (internal/pyjson: dict order, ints of any size, str
// with lone surrogates kept as WTF-8). These are the Python operations bog-write.py applies
// to plan values, each with the exception Python raises on a wrong type.

// pyErr is a Python exception: the last line of its traceback is "<typ>: <msg>".
type pyErr struct{ typ, msg string }

func (e *pyErr) Error() string { return e.typ + ": " + e.msg }

func raise(typ, format string, a ...any) error { return &pyErr{typ, fmt.Sprintf(format, a...)} }

// finding is a validation (severity, message) tuple; severity "ERROR" blocks import.
type finding [2]string

// repr is repr() of the (severity, message) tuple.
func (f finding) repr() string { return pycompat.Repr(pycompat.Tuple{f[0], f[1]}) }

func str(s string) pyjson.Value { return pyjson.String(s) }

// getKey is v.get(key): the value and whether the key exists.
func getKey(v pyjson.Value, key string) (pyjson.Value, bool, error) {
	if v.Kind != pyjson.Object {
		return pyjson.Value{}, false, raise("AttributeError", "'%s' object has no attribute 'get'", v.TypeName())
	}
	x, found := v.Get(key)
	return x, found, nil
}

// get is v.get(key, def).
func get(v pyjson.Value, key string, def pyjson.Value) (pyjson.Value, error) {
	x, found, err := getKey(v, key)
	if err != nil || !found {
		return def, err
	}
	return x, nil
}

// subscript is v[key] with a str key.
func subscript(v pyjson.Value, key string) (pyjson.Value, error) {
	switch v.Kind {
	case pyjson.Object:
		if x, ok := v.Get(key); ok {
			return x, nil
		}
		return pyjson.Value{}, raise("KeyError", "%s", pyjson.StrRepr(key))
	case pyjson.Array:
		return pyjson.Value{}, raise("TypeError", "list indices must be integers or slices, not str")
	case pyjson.Str:
		return pyjson.Value{}, raise("TypeError", "string indices must be integers, not 'str'")
	}
	return pyjson.Value{}, raise("TypeError", "'%s' object is not subscriptable", v.TypeName())
}

// iterate is iter(v): list items, dict keys or the characters of a str.
func iterate(v pyjson.Value) ([]pyjson.Value, error) {
	switch v.Kind {
	case pyjson.Array:
		return v.A, nil
	case pyjson.Object:
		out := make([]pyjson.Value, len(v.Keys))
		for i, k := range v.Keys {
			out[i] = str(k)
		}
		return out, nil
	case pyjson.Str:
		var out []pyjson.Value
		for _, r := range pyjson.Runes(v.S) {
			out = append(out, str(string(pyjson.AppendRune(nil, r))))
		}
		return out, nil
	}
	return nil, raise("TypeError", "'%s' object is not iterable", v.TypeName())
}

// asStr is v as a str for a str method call (AttributeError on any other type).
func asStr(v pyjson.Value, method string) (string, error) {
	if v.Kind == pyjson.Str {
		return v.S, nil
	}
	return "", raise("AttributeError", "'%s' object has no attribute '%s'", v.TypeName(), method)
}

// contains is `sub in v` for a str sub.
func contains(v pyjson.Value, sub string) (bool, error) {
	switch v.Kind {
	case pyjson.Str:
		return strings.Contains(v.S, sub), nil
	case pyjson.Array:
		for _, e := range v.A {
			if e.Kind == pyjson.Str && e.S == sub {
				return true, nil
			}
		}
		return false, nil
	case pyjson.Object:
		_, ok := v.Get(sub)
		return ok, nil
	}
	return false, raise("TypeError", "argument of type '%s' is not a container or iterable", v.TypeName())
}

// pyIntOf is int(v) for a ws coordinate, as decimal text.
func pyIntOf(v pyjson.Value) (string, error) {
	switch v.Kind {
	case pyjson.Bool:
		if v.B {
			return "1", nil
		}
		return "0", nil
	case pyjson.Int:
		return v.I.String(), nil
	case pyjson.Float:
		switch {
		case math.IsInf(v.F, 0):
			return "", raise("OverflowError", "cannot convert float infinity to integer")
		case math.IsNaN(v.F):
			return "", raise("ValueError", "cannot convert float NaN to integer")
		}
		i, _ := new(big.Float).SetFloat64(math.Trunc(v.F)).Int(nil)
		return i.String(), nil
	case pyjson.Str:
		if n, err := pycompat.ParseBigInt(v.S); err == nil {
			return n.String(), nil
		}
		return "", raise("ValueError", "invalid literal for int() with base 10: %s", pyjson.StrRepr(v.S))
	}
	return "", raise("TypeError", "int() argument must be a string, a bytes-like object or a real number, not '%s'", v.TypeName())
}
