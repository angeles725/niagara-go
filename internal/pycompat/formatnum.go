package pycompat

import (
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
