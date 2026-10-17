package argparse

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Python value rendering (str, repr, %-formatting) and str methods with Python semantics,
// so messages and help text match CPython byte for byte.

// pyStr is Python str(v).
func pyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return pyRepr(v)
}

// pyRepr is Python repr(v) for the value kinds a namespace holds.
func pyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return pyFloatRepr(x)
	case string:
		return pyStrRepr(x)
	case []string:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyStrRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprint(v)
}

// pyFloatRepr is repr(float): shortest round-trip digits, exponent form outside [1e-4, 1e16).
func pyFloatRepr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // [-]d[.ddd]e±XX
	sign := ""
	if e[0] == '-' {
		sign, e = "-", e[1:]
	}
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	if exp < -4 || exp >= 16 {
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		es := "+"
		if exp < 0 {
			es, exp = "-", -exp
		}
		return fmt.Sprintf("%s%se%s%02d", sign, m, es, exp)
	}
	if exp < 0 {
		return sign + "0." + strings.Repeat("0", -exp-1) + digits
	}
	if len(digits) <= exp+1 {
		return sign + digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	}
	return sign + digits[:exp+1] + "." + digits[exp+1:]
}

// pyStrRepr is repr(str).
func pyStrRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x7f || unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// pyPercent is `format % params` for a mapping: %(key)s, %(key)r, %(key)d and %%.
func pyPercent(format string, params map[string]any) (string, error) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(format) {
			return "", fmt.Errorf("incomplete format")
		}
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}
		if format[i] != '(' {
			return "", fmt.Errorf("format requires a mapping")
		}
		end := strings.IndexByte(format[i:], ')')
		if end < 0 {
			return "", fmt.Errorf("incomplete format key")
		}
		key := format[i+1 : i+end]
		i += end + 1
		if i >= len(format) {
			return "", fmt.Errorf("incomplete format")
		}
		v, ok := params[key]
		if !ok {
			return "", fmt.Errorf("KeyError: %s", key)
		}
		switch format[i] {
		case 's':
			b.WriteString(pyStr(v))
		case 'r':
			b.WriteString(pyRepr(v))
		case 'd', 'i':
			switch n := v.(type) {
			case int:
				b.WriteString(strconv.Itoa(n))
			case bool:
				b.WriteString(map[bool]string{true: "1", false: "0"}[n])
			case float64:
				b.WriteString(strconv.FormatInt(int64(n), 10))
			default:
				return "", fmt.Errorf("%%d format: a real number is required")
			}
		default:
			return "", fmt.Errorf("unsupported format character %q", format[i])
		}
	}
	return b.String(), nil
}

// isPySpace is str.isspace for one character.
func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pyStrip is str.strip() with no argument.
func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

// isASCIISpace is the re.ASCII \s class.
func isASCIISpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// normalizeSpace is re.sub(r'\s+', ' ', text, flags=re.ASCII).strip().
func normalizeSpace(text string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range text {
		if isASCIISpace(r) {
			if !inSpace {
				b.WriteByte(' ')
			}
			inSpace = true
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	return pyStrip(b.String())
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitlines is str.splitlines(keepends).
func pySplitlines(s string, keepends bool) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isLineBreak(r) {
			i += size
			continue
		}
		end := i + size
		if r == '\r' && end < len(s) && s[end] == '\n' {
			end++
		}
		if keepends {
			lines = append(lines, s[start:end])
		} else {
			lines = append(lines, s[start:i])
		}
		start, i = end, end
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// runeLen is Python len(str).
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// padRight is '%-*s' % (width, s).
func padRight(s string, width int) string {
	if n := runeLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
