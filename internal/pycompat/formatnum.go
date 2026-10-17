package pycompat

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// number is a formatted number split the way CPython's fill_number writes it: sign, prefix
// ("0x"), the integer digits (grouped and zero-padded here), then an optional decimal point
// and the remainder (fraction, exponent, "%", or the character of the 'c' type).
type number struct {
	neg       bool
	prefix    string
	digits    string
	decimal   bool
	remainder string
}

// layout is CPython calc_number_widths + fill_number:
// <lpadding><sign><prefix><spadding><grouped digits><decimal><remainder><rpadding>.
func layout(n number, f *fspec) string {
	sign := ""
	switch {
	case n.neg:
		sign = "-"
	case f.sign == '+' || f.sign == ' ':
		sign = string(f.sign)
	}
	if n.decimal && f.frac != 0 {
		n.remainder = groupFraction(n.remainder, string(f.frac))
	}
	nonDigits := len(sign) + len(n.prefix) + utf8.RuneCountInString(n.remainder)
	if n.decimal {
		nonDigits++
	}
	minWidth := 0
	if f.fill == '0' && f.align == '=' {
		minWidth = int(f.width) - nonDigits
	}
	grouped := ""
	if n.digits != "" {
		sep := ""
		if f.thousands != 0 {
			sep = string(f.thousands)
		}
		grouped = groupDigits(n.digits, minWidth, sep, f.groupLen)
	}
	var lpad, spad, rpad int
	if pad := int(f.width) - nonDigits - len(grouped); pad > 0 {
		switch f.align {
		case '<':
			rpad = pad
		case '^':
			lpad = pad / 2
			rpad = pad - lpad
		case '=':
			spad = pad
		default:
			lpad = pad
		}
	}
	fill := string(f.fill)
	var b strings.Builder
	b.WriteString(strings.Repeat(fill, lpad))
	b.WriteString(sign)
	b.WriteString(n.prefix)
	b.WriteString(strings.Repeat(fill, spad))
	b.WriteString(grouped)
	if n.decimal {
		b.WriteByte('.')
	}
	b.WriteString(n.remainder)
	b.WriteString(strings.Repeat(fill, rpad))
	return b.String()
}

// groupDigits is CPython _PyUnicode_InsertThousandsGrouping: groups of group digits from the
// right joined by sep (no grouping when sep is empty), left-padded with '0' digits (and
// separators) until it is at least minWidth long.
func groupDigits(digits string, minWidth int, sep string, group int) string {
	remaining := len(digits)
	if sep == "" {
		return strings.Repeat("0", max(0, minWidth-remaining)) + digits
	}
	var chunks []string
	for {
		l := min(group, max(remaining, minWidth, 1))
		chars := max(0, min(remaining, l))
		chunks = append(chunks, strings.Repeat("0", max(0, l-remaining))+digits[remaining-chars:remaining])
		remaining -= chars
		minWidth -= l
		if remaining <= 0 && minWidth <= 0 {
			break
		}
		minWidth -= len(sep)
	}
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}
	return strings.Join(chunks, sep)
}

// groupFraction puts sep after every three digits of the fraction that starts remainder.
func groupFraction(remainder, sep string) string {
	end := 0
	for end < len(remainder) && remainder[end] >= '0' && remainder[end] <= '9' {
		end++
	}
	var b strings.Builder
	for i := 0; i < end; i++ {
		if i > 0 && i%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteByte(remainder[i])
	}
	b.WriteString(remainder[end:])
	return b.String()
}

func formatFloat(v float64, typeName, spec string) (string, error) {
	if spec == "" {
		return FloatRepr(v), nil
	}
	f, err := parseSpec(spec, typeName, 0, '>')
	if err != nil {
		return "", err
	}
	switch f.typ {
	case 0, 'e', 'E', 'f', 'F', 'g', 'G', 'n', '%':
		return formatFloatSpec(v, f)
	}
	return "", unknownCode(f.typ, typeName)
}

// formatFloatSpec is CPython format_float_internal.
func formatFloatSpec(v float64, f *fspec) (string, error) {
	if f.precision > math.MaxInt32 {
		return "", valueError("precision too big")
	}
	typ, prec, defPrec, addDot0, pct := f.typ, int(f.precision), 6, false, false
	if typ == 0 {
		typ, defPrec, addDot0 = 'r', 0, true
	}
	if typ == 'n' {
		typ = 'g'
	}
	if typ == '%' {
		typ, v, pct = 'f', v*100, true
	}
	if prec < 0 {
		prec = defPrec
	} else if typ == 'r' {
		typ = 'g'
	}
	s := doubleToString(v, byte(typ), prec, f.alternate, f.noNeg0, addDot0)
	if pct {
		s += "%"
	}
	num := number{}
	if s[0] == '-' {
		num.neg, s = true, s[1:]
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	num.digits, s = s[:i], s[i:]
	if s != "" && s[0] == '.' {
		num.decimal, s = true, s[1:]
	}
	num.remainder = s
	return layout(num, f), nil
}

// doubleToString is CPython PyOS_double_to_string for the codes e E f F g G r, with the
// digits of David Gay's dtoa taken from strconv (both are correctly rounded).
func doubleToString(v float64, code byte, prec int, alt, noNeg0, addDot0 bool) string {
	upper := code == 'E' || code == 'F' || code == 'G'
	if upper {
		code += 'a' - 'A'
	}
	switch {
	case math.IsNaN(v):
		return caseIf(upper, "nan")
	case math.IsInf(v, 0):
		if v < 0 {
			return "-" + caseIf(upper, "inf")
		}
		return caseIf(upper, "inf")
	}
	var digits string
	var decpt int
	switch code {
	case 'e':
		prec++
		digits, decpt = sigDigits(v, prec)
	case 'f':
		digits, decpt = fixedDigits(v, prec)
	case 'g':
		prec = max(prec, 1)
		digits, decpt = sigDigits(v, prec)
	default: // 'r'
		digits, decpt = sigDigits(v, -1)
	}
	neg := math.Signbit(v) && !(noNeg0 && strings.Trim(digits, "0") == "")
	return formatShort(neg, digits, decpt, code, prec, alt, addDot0, upper)
}

func caseIf(upper bool, s string) string {
	if upper {
		return strings.ToUpper(s)
	}
	return s
}

// sigDigits returns n significant digits of |v| (shortest round-trip when n < 0) without
// trailing zeros, and the decimal point position; zero is "0" at 1, like dtoa.
func sigDigits(v float64, n int) (string, int) {
	if n > 0 {
		n--
	}
	mant, exp, _ := strings.Cut(strconv.FormatFloat(math.Abs(v), 'e', n, 64), "e")
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	e, _ := strconv.Atoi(exp)
	if digits == "" {
		return "0", 1
	}
	return digits, e + 1
}

// fixedDigits returns the digits of |v| rounded to n decimals without leading or trailing
// zeros, and the decimal point position; a result of zero is "0" at 1.
func fixedDigits(v float64, n int) (string, int) {
	s := strconv.FormatFloat(math.Abs(v), 'f', n, 64)
	intLen := strings.IndexByte(s, '.')
	if intLen < 0 {
		intLen = len(s)
	}
	all := strings.Replace(s, ".", "", 1)
	trimmed := strings.TrimLeft(all, "0")
	if strings.TrimRight(trimmed, "0") == "" {
		return "0", 1
	}
	return strings.TrimRight(trimmed, "0"), intLen - (len(all) - len(trimmed))
}

// formatShort is CPython format_float_short after the dtoa call: it lays out the digit
// slice vdigits[vstart:vend] of digits padded with zeros on both sides, the decimal point
// and the exponent.
func formatShort(neg bool, digits string, decpt int, code byte, prec int, alt, addDot0, upper bool) string {
	useExp, exp, vend := false, 0, len(digits)
	switch code {
	case 'e':
		useExp, vend = true, prec
	case 'f':
		vend = decpt + prec
	case 'g':
		limit := prec
		if addDot0 {
			limit--
		}
		useExp = decpt <= -4 || decpt > limit
		if alt {
			vend = prec
		}
	case 'r':
		useExp = decpt <= -4 || decpt > 16
	}
	if useExp {
		exp, decpt = decpt-1, 1
	}
	vstart := 0
	if decpt <= 0 {
		vstart = decpt - 1
	}
	if !useExp && addDot0 {
		vend = max(vend, decpt+1)
	} else {
		vend = max(vend, decpt)
	}
	zeros := func(n int) string { return strings.Repeat("0", max(0, n)) }
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	if decpt <= 0 {
		b.WriteString(zeros(decpt-vstart) + "." + zeros(-decpt))
	} else {
		b.WriteString(zeros(-vstart))
	}
	if n := len(digits); 0 < decpt && decpt <= n {
		b.WriteString(digits[:decpt] + "." + digits[decpt:])
	} else {
		b.WriteString(digits)
	}
	if n := len(digits); n < decpt {
		b.WriteString(zeros(decpt-n) + "." + zeros(vend-decpt))
	} else {
		b.WriteString(zeros(vend - n))
	}
	s := b.String()
	if !alt && strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
	}
	if useExp {
		e := "e"
		if upper {
			e = "E"
		}
		s += fmt.Sprintf("%s%+03d", e, exp)
	}
	return s
}
