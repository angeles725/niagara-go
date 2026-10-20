package pycompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Tuple is a Python tuple for Repr and Str: "(a,)" for one item.
type Tuple []any

// Reprer is a value that knows its Python repr(); Repr and Str use PyRepr for it, also
// inside []any and Tuple, ahead of every other rule (fmt.Stringer included).
type Reprer interface{ PyRepr() string }

// OrderedMap is a dict in insertion order (a decoded JSON object): Values()[i] belongs to
// Keys()[i]. Repr shows it as {'k': v, ...}.
type OrderedMap interface {
	Keys() []string
	Values() []any
}

// Repr is Python repr(v) for nil (None), a Reprer, bool, the Go integer kinds, *big.Int,
// float64, float32, string, []string, []any, Tuple, *string (Optional[str]), json.Number (the
// int or float json.loads gives it), OrderedMap and fmt.Stringer.
func Repr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case Reprer:
		return x.PyRepr()
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case *big.Int:
		return x.String()
	case float64:
		return FloatRepr(x)
	case float32:
		return FloatRepr(float64(x))
	case string:
		return StrRepr(x)
	case *string:
		return OptRepr(x)
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = StrRepr(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		return "[" + reprItems(x) + "]"
	case Tuple:
		if len(x) == 1 {
			return "(" + Repr(x[0]) + ",)"
		}
		return "(" + reprItems(x) + ")"
	case json.Number:
		return numberRepr(x)
	case OrderedMap:
		keys, vals := x.Keys(), x.Values()
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = StrRepr(k) + ": " + Repr(vals[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}

// numberRepr is repr() of what json.loads makes of n: an int (any size) for a literal
// without '.', exponent, NaN or Infinity, otherwise a float (out of range is ±inf).
func numberRepr(n json.Number) string {
	s := string(n)
	if !strings.ContainsAny(s, ".eEIN") {
		if i, ok := new(big.Int).SetString(s, 10); ok {
			return i.String()
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		f = 0
	}
	return FloatRepr(f)
}

func reprItems(items []any) string {
	parts := make([]string, len(items))
	for i, e := range items {
		parts[i] = Repr(e)
	}
	return strings.Join(parts, ", ")
}

// Str is Python str(v): a string as is, anything else as Repr.
func Str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case *string:
		return OptStr(x)
	}
	return Repr(v)
}

// OptRepr is repr() of an Optional[str]: None or the quoted text.
func OptRepr(s *string) string {
	if s == nil {
		return "None"
	}
	return StrRepr(*s)
}

// OptStr is str() (or an f-string field) of an Optional[str]: None or the text.
func OptStr(s *string) string {
	if s == nil {
		return "None"
	}
	return *s
}

// StrRepr is repr(str): single quotes unless the text has a single quote and no double
// quote; backslash, the quote, \t \n \r and non-printable characters escaped.
func StrRepr(s string) string { return strRepr(s, false) }

// FSRepr is repr(os.fsdecode(p)) for a file name: StrRepr, with each byte that is not
// valid UTF-8 shown as the \udcXX surrogate escape Python's file-system decoding gives it.
func FSRepr(p string) string { return strRepr(p, true) }

func strRepr(s string, surrogates bool) string {
	quote := byte('\'')
	if strings.IndexByte(s, '\'') >= 0 && strings.IndexByte(s, '"') < 0 {
		quote = '"'
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte(quote)
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case surrogates && r == utf8.RuneError && n == 1:
			fmt.Fprintf(&b, `\udc%02x`, s[i])
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case IsPrintable(r):
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
		i += n
	}
	b.WriteByte(quote)
	return b.String()
}

// FloatRepr is repr(float): shortest round-trip digits, positional for exponents -4..15
// (with ".0" when integral), otherwise d.ddde±XX.
func FloatRepr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
	}
	mant, expStr, _ := strings.Cut(strconv.FormatFloat(math.Abs(f), 'e', -1, 64), "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	switch {
	case exp < -4 || exp >= 16:
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		return fmt.Sprintf("%s%se%+03d", sign, m, exp)
	case exp < 0:
		return sign + "0." + strings.Repeat("0", -exp-1) + digits
	case len(digits) <= exp+1:
		return sign + digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	}
	return sign + digits[:exp+1] + "." + digits[exp+1:]
}

// FormatG is "%g" % f: six significant digits, trailing zeros dropped, e±XX exponents.
func FormatG(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// JSONFloat is json.dumps(f): repr, with NaN, Infinity and -Infinity spelled out.
func JSONFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return FloatRepr(f)
}

// IntOfFloat is str(int(f)) for a finite f (any magnitude); ok is false for nan and inf,
// where Python raises.
func IntOfFloat(f float64) (string, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", false
	}
	i, _ := big.NewFloat(f).Int(nil)
	return i.String(), true
}
