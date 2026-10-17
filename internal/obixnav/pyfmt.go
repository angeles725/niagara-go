package obixnav

// Python value formatting shared by the commands. pyRepr, decodeReplace and pyFloat are copies of
// the hdbread and pxrender helpers (a tool never imports another tool's package); they belong in a
// shared internal package once one exists.

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// pyRepr renders a float64 exactly like Python repr(): shortest round-trip digits, positional
// notation for exponents -4..15 (with ".0" when integral), otherwise d.ddde+XX.
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

// decodeReplace is Python bytes.decode("utf-8", "replace"): each maximal invalid subpart of a
// sequence becomes one U+FFFD.
func decodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			sb.WriteByte(c)
			i++
			continue
		}
		need, lo, hi := 0, byte(0x80), byte(0xbf)
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need, lo = 2, 0xa0
		case c >= 0xe1 && c <= 0xef && c != 0xed:
			need = 2
		case c == 0xed:
			need, hi = 2, 0x9f
		case c == 0xf0:
			need, lo = 3, 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need, hi = 3, 0x8f
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lo || b[j] > hi {
				break
			}
			j++
			lo, hi = 0x80, 0xbf
		}
		if need > 0 && j == i+1+need {
			sb.Write(b[i:j])
		} else {
			sb.WriteRune('�')
		}
		i = j
	}
	return sb.String()
}

// isPySpace is Python str.isspace: Go's unicode.IsSpace plus the four ASCII separators 0x1c-0x1f.
func isPySpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

var pyFloatRE = regexp.MustCompile(`^[+-]?(?:[0-9](?:_?[0-9])*(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)` +
	`(?:[eE][+-]?[0-9](?:_?[0-9])*)?$`)

// pyFloat parses like Python float(str): surrounding whitespace, underscores between digits,
// inf/infinity/nan in any case; no hex.
func pyFloat(s string) (float64, bool) {
	s = pyStrip(s)
	body, neg := s, false
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body, neg = body[1:], body[0] == '-'
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if neg {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	if !pyFloatRE.MatchString(s) {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return v, true
}

// pyStrRepr is Python repr(str): single quotes unless the text has a single quote and no double
// quote; backslash, the quote, \t \n \r and non-printable characters escaped.
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

// optRepr is repr() of an Optional[str]: None or the quoted text.
func optRepr(s *string) string {
	if s == nil {
		return "None"
	}
	return pyStrRepr(*s)
}

// optStr is "%s" of an Optional[str].
func optStr(s *string) string {
	if s == nil {
		return "None"
	}
	return *s
}

func pyListRepr(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = pyStrRepr(s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func pyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
