package core

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Python formatting the commands share. Widths and cuts count code points,
// like str.format and slicing.

// PyStr is str(v) of a decoded JSON value.
func PyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return PyRepr(v)
}

// PyRepr is repr(v) of a decoded JSON value: None, True, ints, float repr,
// quoted strings, lists and dicts.
func PyRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return pycompat.StrRepr(t)
	case json.Number:
		return numberRepr(t)
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = PyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *index.Object:
		parts := make([]string, t.Len())
		for i, k := range t.Keys() {
			parts[i] = pycompat.StrRepr(k) + ": " + PyRepr(t.Values()[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "<?>"
}

func numberRepr(n json.Number) string {
	if index.IsInt(n) {
		if i, ok := index.Int(n); ok {
			return strconv.FormatInt(i, 10)
		}
		return string(n) // beyond int64: Python prints the literal digits
	}
	f, _ := index.Float(n)
	return pycompat.FloatRepr(f)
}

// Commas is "{:,}".format(n) (pycompat.Commas).
func Commas(n int64) string { return pycompat.Commas(n) }

// Fixed is "{:.Nf}".format(f) (round half to even on exact ties, like Python).
func Fixed(f float64, prec int) string { return strconv.FormatFloat(f, 'f', prec, 64) }

// LJust is "{:<w}"/"{:w}" of a string: padded on the right to w code points.
func LJust(s string, w int) string { return pycompat.LJust(s, w, ' ') }

// RJust is "{:>w}": padded on the left to w code points.
func RJust(s string, w int) string { return pycompat.RJust(s, w, ' ') }

// Cut is s[:n] in code points.
func Cut(s string, n int) string { return pycompat.Cut(s, n) }
