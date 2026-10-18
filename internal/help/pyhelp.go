package help

// Python built-in behaviours the lookups print through: str() and bool() of
// decoded JSON values, dict.get, "{:,}", code-point padding and slicing,
// str.lower and text-mode reads. They cover decoded JSON values and print
// formats internal/pycompat does not (its Percent has no width or flags);
// str.strip and html.unescape come from pycompat.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// pyStr is str(v) for a decoded JSON value, as format() prints it.
func pyStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

// get is o.get(k, def); a nil object is an empty dict.
func get(o *index.Object, k string, def any) any {
	if o == nil {
		return def
	}
	if v, ok := o.Get(k); ok {
		return v
	}
	return def
}

// getStr is str(o.get(k, def)).
func getStr(o *index.Object, k, def string) string { return pyStr(get(o, k, def)) }

// orStr is `o.get(k, def) or alt`.
func orStr(o *index.Object, k, def, alt string) string {
	v := get(o, k, def)
	if !index.Truthy(v) {
		return alt
	}
	return pyStr(v)
}

// truthyStr is str(o.get(k)) when that value is truthy, else "": the
// `if not value` tests Python makes before using a value that may be null.
func truthyStr(o *index.Object, k string) string { return orStr(o, k, "", "") }

// getObj is o.get(k, {}) when the value is an object (nil means empty).
func getObj(o *index.Object, k string) *index.Object {
	if o == nil {
		return nil
	}
	return o.Object(k)
}

// strList is o.get(k, []) as strings; a non-list value is empty.
func strList(o *index.Object, k string) []string {
	arr, _ := get(o, k, nil).([]any)
	out := make([]string, len(arr))
	for i, v := range arr {
		out[i] = pyStr(v)
	}
	return out
}

// listLen is len(o.get(k, [])).
func listLen(o *index.Object, k string) int {
	arr, _ := get(o, k, nil).([]any)
	return len(arr)
}

// thousands is "{:,}".format(n).
func thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// runeLen is len(s) for a Python str.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// ljust is "{:<w}".format(s): padded by code points, never truncated.
func ljust(s string, w int) string {
	if n := runeLen(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// rjust is "{:>w}".format(s).
func rjust(s string, w int) string {
	if n := runeLen(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// headRunes is s[:n] by code points.
func headRunes(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// lowerStr is str.lower().
func lowerStr(s string) string { return textfold.Lower(s) }

// readText is open(path, encoding="utf-8").read(): universal newlines turn
// \r\n and \r into \n. ok is false when path is not a regular file.
func readText(path string) (text string, ok bool, err error) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return "", false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return pycompat.UniversalNewlines(string(data)), true, nil
}

// pyHead is items[:n] with Python's slice bounds: a negative n counts from
// the end, and the result is clamped to the list.
func pyHead[T any](items []T, n int) []T {
	if n < 0 {
		n += len(items)
	}
	return items[:max(0, min(n, len(items)))]
}

// pyInt is a decoded JSON integer as a Go int (0 for anything else).
func pyInt(v any) int {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}
