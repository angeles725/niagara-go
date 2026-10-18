package index

// Truthy is Python's bool(v) for a value Decode returns: false for null,
// false, "", a zero number, [] and {}; true for everything else. A number is
// judged by the value json.loads gives for its literal, so -0 and 1e-400
// (0.0) are false while NaN, ±Infinity, 1e400 (inf) and integers beyond
// float64 are true.
//
// Go numbers (int, int64, float64), which callers may pass for computed
// values, follow Python too: true unless zero (NaN is true). Values with no
// Python counterpart are false: a nil *Object (an absent dict) and a
// json.Number that is not a JSON number literal.
func Truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case *Object:
		return t != nil && t.Len() > 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	}
	f, ok := Float(v)
	return ok && f != 0
}
