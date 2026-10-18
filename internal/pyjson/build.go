package pyjson

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Building a Value from Go data: the constructors below build one directly, and
// FromAny reads the Go values the ports hold (decoded index values, dict literals,
// slices and scalars), so every port prints json.dumps through Dumps.

// Field is one member of a dict literal built with ObjectOf.
type Field struct {
	Key   string
	Value Value
}

// ObjectOf is the dict literal {key: value, ...} in field order. A repeated key keeps
// its first position and takes its last value, as in Python.
func ObjectOf(fields ...Field) Value {
	o := NewObject()
	for _, f := range fields {
		o.Set(f.Key, f.Value)
	}
	return o
}

// BoolValue builds a bool value.
func BoolValue(b bool) Value { return Value{Kind: Bool, B: b} }

// FloatValue builds a float value.
func FloatValue(f float64) Value { return Value{Kind: Float, F: f} }

// StringList builds a list of str values.
func StringList(ss []string) Value {
	a := make([]Value, len(ss))
	for i, s := range ss {
		a[i] = String(s)
	}
	return Value{Kind: Array, A: a}
}

// KV is one member of an Obj. It aliases an unnamed struct so other packages can
// write Obj{{"key", v}} without go vet's unkeyed-field report for imported structs.
type KV = struct {
	Key string
	Val any
}

// Obj is a dict literal of Go values for FromAny: its keys print in order.
type Obj []KV

// Dict is an ordered mapping FromAny reads as a dict: Values()[i] belongs to
// Keys()[i]. *index.Object is one; a nil pointer reads as {}.
type Dict interface {
	Keys() []string
	Values() []any
}

// FromAny converts v to a Value. It reads nil (None), bool, int, int64, float64,
// *float64 (nil is None), string, json.Number (a literal with a fraction, an exponent,
// NaN or Infinity is a float, any other an int of any size), []string, []any, Obj, Dict
// and Value (kept as is). A Go string, an Obj key included, is read as Go text: each
// byte of invalid UTF-8 is one U+FFFD; a WTF-8 string with lone surrogates must come in
// as a Value. Any other type is an error.
func FromAny(v any) (Value, error) {
	switch x := v.(type) {
	case nil:
		return Value{}, nil
	case Value:
		return x, nil
	case bool:
		return BoolValue(x), nil
	case int:
		return IntValue(int64(x)), nil
	case int64:
		return IntValue(x), nil
	case float64:
		return FloatValue(x), nil
	case *float64:
		if x == nil {
			return Value{}, nil
		}
		return FloatValue(*x), nil
	case string:
		return String(goText(x)), nil
	case json.Number:
		return numberValue(string(x))
	case []string:
		a := make([]Value, len(x))
		for i, s := range x {
			a[i] = String(goText(s))
		}
		return Value{Kind: Array, A: a}, nil
	case []any:
		a := make([]Value, len(x))
		for i, it := range x {
			var err error
			if a[i], err = FromAny(it); err != nil {
				return Value{}, err
			}
		}
		return Value{Kind: Array, A: a}, nil
	case Obj:
		o := NewObject()
		for _, kv := range x {
			val, err := FromAny(kv.Val)
			if err != nil {
				return Value{}, err
			}
			o.Set(goText(kv.Key), val)
		}
		return o, nil
	case Dict:
		return dictValue(x)
	}
	return Value{}, fmt.Errorf("pyjson.FromAny: unsupported %T", v)
}

// DumpsAny is Dumps(FromAny(v), o). It panics when FromAny fails: the callers hold
// values of the types FromAny reads by construction.
func DumpsAny(v any, o DumpOpts) string {
	val, err := FromAny(v)
	if err != nil {
		panic(err)
	}
	return Dumps(val, o)
}

func dictValue(d Dict) (Value, error) {
	o := NewObject()
	if rv := reflect.ValueOf(d); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return o, nil
	}
	vals := d.Values()
	for i, k := range d.Keys() {
		val, err := FromAny(vals[i])
		if err != nil {
			return Value{}, err
		}
		o.Set(goText(k), val)
	}
	return o, nil
}

// numberValue is the Python value json.loads gives the number literal lit.
func numberValue(lit string) (Value, error) {
	if !strings.ContainsAny(lit, ".eEIN") {
		i, ok := new(big.Int).SetString(lit, 10)
		if !ok {
			return Value{}, fmt.Errorf("pyjson.FromAny: bad int literal %q", lit)
		}
		return Value{Kind: Int, I: i}, nil
	}
	f, err := strconv.ParseFloat(lit, 64)
	if ne, ok := err.(*strconv.NumError); err != nil && (!ok || ne.Err != strconv.ErrRange) {
		return Value{}, fmt.Errorf("pyjson.FromAny: bad float literal %q", lit)
	}
	return FloatValue(f), nil // out of range: ±Inf, as float() gives
}

// goText is s as Go's range reads it: unchanged when valid UTF-8, else each invalid
// byte (a WTF-8 surrogate's three included) becomes one U+FFFD.
func goText(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return string([]rune(s))
}
