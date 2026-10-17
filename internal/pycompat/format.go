package pycompat

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode/utf8"
)

// FormatError is the exception format() raises: Type is the Python class (ValueError,
// OverflowError or TypeError) and Msg its exact str().
type FormatError struct{ Type, Msg string }

func (e *FormatError) Error() string { return e.Msg }

func valueError(format string, a ...any) error {
	return &FormatError{"ValueError", fmt.Sprintf(format, a...)}
}

// fspec is CPython's InternalFormatSpec: one parsed format_spec.
type fspec struct {
	fill, align, sign rune
	noNeg0, alternate bool
	width, precision  int64 // -1 when absent
	thousands, frac   rune  // 0, ',' or '_'
	groupLen          int   // 3, or 4 for '_' with b, o, x and X
	typ               rune  // 0 when absent and the type has no default
}

// Format is Python's format(v, spec) — what str.format and f-string fields do — for nil
// (None), bool, the Go integer kinds, *big.Int and string, following the
// Python 3.15 format-spec mini-language: [[fill]align][sign][z][#][0][width][grouping]
// [.precision[grouping]][type]. Widths and precisions count code points. Errors are
// *FormatError with Python's exception class and message. Floats and the float presentation
// types (e, E, f, F, g, G, %) are not supported yet and fail with a TypeError.
func Format(v any, spec string) (string, error) {
	switch x := v.(type) {
	case nil:
		if spec == "" {
			return "None", nil
		}
		return "", &FormatError{"TypeError", "unsupported format string passed to NoneType.__format__"}
	case string:
		return formatStr(x, spec)
	case bool:
		if spec == "" {
			return Str(x), nil
		}
		n := big.NewInt(0)
		if x {
			n.SetInt64(1)
		}
		return formatInt(n, "bool", spec)
	case *big.Int:
		return formatInt(x, "int", spec)
	}
	if n, ok := goInt(v); ok {
		return formatInt(n, "int", spec)
	}
	return "", &FormatError{"TypeError", fmt.Sprintf("pycompat.Format: unsupported Go type %T", v)}
}

func goInt(v any) (*big.Int, bool) {
	switch x := v.(type) {
	case int:
		return big.NewInt(int64(x)), true
	case int8:
		return big.NewInt(int64(x)), true
	case int16:
		return big.NewInt(int64(x)), true
	case int32:
		return big.NewInt(int64(x)), true
	case int64:
		return big.NewInt(x), true
	case uint:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint8:
		return big.NewInt(int64(x)), true
	case uint16:
		return big.NewInt(int64(x)), true
	case uint32:
		return big.NewInt(int64(x)), true
	case uint64:
		return new(big.Int).SetUint64(x), true
	}
	return nil, false
}

func isAlign(r rune) bool { return r == '<' || r == '>' || r == '=' || r == '^' }

// readInt is CPython get_integer: a run of Unicode decimal digits; n is how many were read.
func readInt(rs []rune, pos *int) (v int64, n int, err error) {
	for ; *pos < len(rs); *pos++ {
		d := decimalValue(rs[*pos])
		if d < 0 {
			break
		}
		if v > (math.MaxInt64-int64(d))/10 {
			return 0, 0, valueError("Too many decimal digits in format string")
		}
		v = v*10 + int64(d)
		n++
	}
	return v, n, nil
}

// codeText renders a presentation type the way CPython's error messages do.
func codeText(r rune) string {
	if r > 32 && r < 128 {
		return string(r)
	}
	return fmt.Sprintf(`\x%x`, r)
}

func sepTypeError(sep, typ rune) error {
	return valueError("Cannot specify '%c' with '%s'.", sep, codeText(typ))
}

// parseSpec is CPython parse_internal_render_format_spec.
func parseSpec(spec, typeName string, defType, defAlign rune) (*fspec, error) {
	rs := []rune(spec)
	f := &fspec{fill: ' ', align: defAlign, width: -1, precision: -1, groupLen: 3, typ: defType}
	pos, fillGiven, alignGiven := 0, false, false
	if len(rs) >= 2 && isAlign(rs[1]) {
		f.fill, f.align, pos, fillGiven, alignGiven = rs[0], rs[1], 2, true, true
	} else if len(rs) >= 1 && isAlign(rs[0]) {
		f.align, pos, alignGiven = rs[0], 1, true
	}
	at := func(r rune) bool { return pos < len(rs) && rs[pos] == r }
	if pos < len(rs) && (rs[pos] == '+' || rs[pos] == '-' || rs[pos] == ' ') {
		f.sign = rs[pos]
		pos++
	}
	if at('z') {
		f.noNeg0 = true
		pos++
	}
	if at('#') {
		f.alternate = true
		pos++
	}
	if !fillGiven && at('0') {
		f.fill = '0'
		if !alignGiven && defAlign == '>' {
			f.align = '='
		}
		pos++
	}
	w, n, err := readInt(rs, &pos)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		f.width = w
	}
	if err := parseGrouping(rs, &pos, &f.thousands); err != nil {
		return nil, err
	}
	if at('.') {
		pos++
		p, n, err := readInt(rs, &pos)
		if err != nil {
			return nil, err
		}
		f.precision = p
		start := pos
		if err := parseGrouping(rs, &pos, &f.frac); err != nil {
			return nil, err
		}
		if n == 0 {
			f.precision = -1
			if pos == start {
				return nil, valueError("Format specifier missing precision")
			}
		}
	}
	switch len(rs) - pos {
	case 0:
	case 1:
		f.typ = rs[pos]
	default:
		return nil, valueError("Invalid format specifier '%s' for object of type '%s'", spec, typeName)
	}
	if f.thousands != 0 {
		switch f.typ {
		case 'd', 'e', 'f', 'g', 'E', 'G', '%', 'F', 0:
		case 'b', 'o', 'x', 'X':
			if f.thousands != '_' {
				return nil, sepTypeError(f.thousands, f.typ)
			}
			f.groupLen = 4
		default:
			return nil, sepTypeError(f.thousands, f.typ)
		}
	}
	if f.frac != 0 && f.typ == 'n' {
		return nil, sepTypeError(f.frac, f.typ)
	}
	return f, nil
}

// parseGrouping reads an optional ',' or '_' (both, in either order, is an error).
func parseGrouping(rs []rune, pos *int, sep *rune) error {
	if *pos < len(rs) && rs[*pos] == ',' {
		*sep = ','
		*pos++
	}
	if *pos < len(rs) && rs[*pos] == '_' {
		if *sep != 0 {
			return valueError("Cannot specify both ',' and '_'.")
		}
		*sep = '_'
		*pos++
	}
	if *pos < len(rs) && rs[*pos] == ',' && *sep == '_' {
		return valueError("Cannot specify both ',' and '_'.")
	}
	return nil
}

func unknownCode(typ rune, typeName string) error {
	return valueError("Unknown format code '%s' for object of type '%s'", codeText(typ), typeName)
}

func formatStr(s, spec string) (string, error) {
	if spec == "" {
		return s, nil
	}
	f, err := parseSpec(spec, "str", 's', '<')
	if err != nil {
		return "", err
	}
	switch {
	case f.typ != 's':
		return "", unknownCode(f.typ, "str")
	case f.sign == ' ':
		return "", valueError("Space not allowed in string format specifier")
	case f.sign != 0:
		return "", valueError("Sign not allowed in string format specifier")
	case f.noNeg0:
		return "", valueError("Negative zero coercion (z) not allowed in string format specifier")
	case f.alternate:
		return "", valueError("Alternate form (#) not allowed in string format specifier")
	case f.align == '=':
		return "", valueError("'=' alignment not allowed in string format specifier")
	}
	n := int64(utf8.RuneCountInString(s))
	if f.precision >= 0 && n > f.precision {
		s, n = Cut(s, int(f.precision)), f.precision
	}
	if f.width <= n {
		return s, nil
	}
	pad := int(f.width - n)
	left := 0
	switch f.align {
	case '>':
		left = pad
	case '^':
		left = pad / 2
	}
	fill := string(f.fill)
	return strings.Repeat(fill, left) + s + strings.Repeat(fill, pad-left), nil
}

func formatInt(n *big.Int, typeName, spec string) (string, error) {
	if spec == "" {
		return n.String(), nil
	}
	f, err := parseSpec(spec, typeName, 'd', '>')
	if err != nil {
		return "", err
	}
	switch f.typ {
	case 'b', 'c', 'd', 'o', 'x', 'X', 'n':
		return formatLong(n, f)
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
		return "", &FormatError{"TypeError", "pycompat.Format: float presentation types are not supported yet"}
	}
	return "", unknownCode(f.typ, typeName)
}

// formatLong is CPython format_long_internal.
func formatLong(n *big.Int, f *fspec) (string, error) {
	switch {
	case f.precision != -1:
		return "", valueError("Precision not allowed in integer format specifier")
	case f.noNeg0:
		return "", valueError("Negative zero coercion (z) not allowed in integer format specifier")
	}
	num := number{neg: n.Sign() < 0}
	if f.typ == 'c' {
		switch {
		case f.sign != 0:
			return "", valueError("Sign not allowed with integer format specifier 'c'")
		case f.alternate:
			return "", valueError("Alternate form (#) not allowed with integer format specifier 'c'")
		case !n.IsInt64():
			return "", &FormatError{"OverflowError", "Python int too large to convert to C long"}
		case n.Int64() < 0 || n.Int64() > 0x10ffff:
			return "", &FormatError{"OverflowError", "%c arg not in range(0x110000)"}
		}
		num.neg, num.remainder = false, string(rune(n.Int64()))
		return layout(num, f), nil
	}
	base, prefix := 10, ""
	switch f.typ {
	case 'b':
		base, prefix = 2, "0b"
	case 'o':
		base, prefix = 8, "0o"
	case 'x':
		base, prefix = 16, "0x"
	case 'X':
		base, prefix = 16, "0X"
	}
	if f.alternate {
		num.prefix = prefix
	}
	num.digits = new(big.Int).Abs(n).Text(base)
	if f.typ == 'X' {
		num.digits = strings.ToUpper(num.digits)
	}
	return layout(num, f), nil
}
