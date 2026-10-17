package greenmaxtablero

// Python text semantics the transform relies on. pyStrRepr and pyOSError are copies of the
// greenmaxmatrix helpers and pyRepr of the obixnav one (a tool never imports another tool's
// package); they belong in a shared internal package once one exists.

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// pyWSChars are the characters of Python's Unicode \s, as RE2 class content.
const pyWSChars = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// pyRe rewrites every \s of a pattern as Python's Unicode whitespace class.
func pyRe(pattern string) string { return strings.ReplaceAll(pattern, `\s`, `[`+pyWSChars+`]`) }

// isPySpace is Python str.isspace.
func isPySpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

// head is s[:n] counted in code points.
func head(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// pyOSError renders an OS error like str(OSError): "[Errno 2] No such file or directory: 'p'".
func pyOSError(err error) string {
	var pe *fs.PathError
	var errno syscall.Errno
	if !errors.As(err, &pe) || !errors.As(pe.Err, &errno) {
		return err.Error()
	}
	msg := errno.Error()
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pyStrRepr(pe.Path))
}

// pyStrRepr is Python repr(str).
func pyStrRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == '\\' || r == rune(q):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case unicode.IsPrint(r):
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// pyRepr renders a float64 exactly like Python repr().
func pyRepr(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(v) {
		sign = "-"
	}
	mant, expPart, _ := strings.Cut(strconv.FormatFloat(math.Abs(v), 'e', -1, 64), "e")
	exp, _ := strconv.Atoi(expPart)
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
	default:
		return sign + digits[:exp+1] + "." + digits[exp+1:]
	}
}
