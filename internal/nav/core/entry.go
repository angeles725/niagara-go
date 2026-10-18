package core

import (
	"github.com/angeles725/niagara-go/internal/index"
)

// Field access on index entries with Python's dict semantics: e[k] printed
// with str(), tested with bool(), counted with len().

// Str is str(o.get(k)): "None" for a missing key or null.
func Str(o *index.Object, k string) string {
	v, _ := o.Get(k)
	return PyStr(v)
}

// Truthy is bool(o.get(k)).
func Truthy(o *index.Object, k string) bool {
	v, _ := o.Get(k)
	return index.Truthy(v)
}

// Int is o[k] as an int (0 when missing or not an integer).
func Int(o *index.Object, k string) int64 {
	v, _ := o.Get(k)
	if n, ok := index.Int(v); ok {
		return n
	}
	if b, ok := v.(bool); ok && b {
		return 1
	}
	return 0
}

// Float is o[k] as a float (0 when missing or not a number).
func Float(o *index.Object, k string) float64 {
	v, _ := o.Get(k)
	f, _ := index.Float(v)
	return f
}

// List is o[k] as a list (nil when missing or not a list).
func List(o *index.Object, k string) []any {
	v, _ := o.Get(k)
	l, _ := v.([]any)
	return l
}

// Strs is [str(x) for x in o[k]].
func Strs(o *index.Object, k string) []string {
	l := List(o, k)
	out := make([]string, len(l))
	for i, x := range l {
		out[i] = PyStr(x)
	}
	return out
}

// Entries is o[k] as a list of objects (non-objects skipped).
func Entries(o *index.Object, k string) []*index.Object {
	return Objects(func() any { v, _ := o.Get(k); return v }())
}

// Objects returns the objects of a decoded list value.
func Objects(v any) []*index.Object {
	l, _ := v.([]any)
	out := make([]*index.Object, 0, len(l))
	for _, x := range l {
		if e, ok := x.(*index.Object); ok {
			out = append(out, e)
		}
	}
	return out
}
