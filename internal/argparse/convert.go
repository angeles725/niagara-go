package argparse

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var errValue = errors.New("invalid literal")

// digitValue is the decimal value of a Unicode Nd character (runs of ten from zero).
func digitValue(r rune) int {
	if r >= '0' && r <= '9' {
		return int(r - '0')
	}
	start := r
	for unicode.IsDigit(start - 1) {
		start--
	}
	return int(r-start) % 10
}

// ParseInt is Python int(text): surrounding whitespace, an optional sign, decimal digits
// (any Unicode Nd) with single underscores between them. Values beyond int64 are rejected.
func ParseInt(text string) (int, error) {
	s := pyStrip(text)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" {
		return 0, errValue
	}
	var n int64
	prevUnderscore, seen := true, false
	for _, r := range s {
		if r == '_' {
			if prevUnderscore {
				return 0, errValue
			}
			prevUnderscore = true
			continue
		}
		if !unicode.IsDigit(r) {
			return 0, errValue
		}
		d := int64(digitValue(r))
		if n > (math.MaxInt64-d)/10 {
			return 0, errValue
		}
		n = n*10 + d
		prevUnderscore, seen = false, true
	}
	if prevUnderscore || !seen {
		return 0, errValue
	}
	if neg {
		n = -n
	}
	return int(n), nil
}

var floatSyntax = regexp.MustCompile(`^[+-]?(?:` +
	`(?:\d(?:_?\d)*(?:\.(?:\d(?:_?\d)*)?)?|\.\d(?:_?\d)*)(?:[eE][+-]?\d(?:_?\d)*)?` +
	`|(?i:inf|infinity|nan))$`)

// ParseFloat is Python float(text) for ASCII digits: decimal and exponent forms, inf,
// infinity and nan in any case, underscores between digits; overflow gives ±inf.
func ParseFloat(text string) (float64, error) {
	s := pyStrip(text)
	if !floatSyntax.MatchString(s) {
		return 0, errValue
	}
	if l := strings.ToLower(strings.TrimLeft(s, "+-")); l == "nan" {
		return math.NaN(), nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, errValue
	}
	return f, nil
}
