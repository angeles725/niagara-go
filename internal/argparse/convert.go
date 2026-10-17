package argparse

import (
	"errors"
	"math"
	"math/big"
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

// ParseBigInt is Python int(text): surrounding whitespace, an optional sign, decimal digits
// (any Unicode Nd) with single underscores between them, any magnitude.
func ParseBigInt(text string) (*big.Int, error) {
	s := pyStrip(text)
	var digits strings.Builder
	if s != "" && (s[0] == '+' || s[0] == '-') {
		digits.WriteByte(s[0])
		s = s[1:]
	}
	prevUnderscore, seen := true, false
	for _, r := range s {
		if r == '_' {
			if prevUnderscore {
				return nil, errValue
			}
			prevUnderscore = true
			continue
		}
		if !unicode.IsDigit(r) {
			return nil, errValue
		}
		digits.WriteByte(byte('0' + digitValue(r)))
		prevUnderscore, seen = false, true
	}
	if prevUnderscore || !seen {
		return nil, errValue
	}
	n, _ := new(big.Int).SetString(digits.String(), 10)
	return n, nil
}

// ErrRange reports an int() literal that is valid but does not fit in an int64.
var ErrRange = errors.New("int out of int64 range")

// ParseInt is ParseBigInt for values that fit in an int64 (ErrRange otherwise).
func ParseInt(text string) (int64, error) {
	n, err := ParseBigInt(text)
	if err != nil {
		return 0, err
	}
	if !n.IsInt64() {
		return 0, ErrRange
	}
	return n.Int64(), nil
}

// intValue is the namespace value of type=int: an int, or a *big.Int beyond int's range.
func intValue(text string) (any, error) {
	n, err := ParseBigInt(text)
	if err != nil {
		return nil, err
	}
	if n.IsInt64() && int64(int(n.Int64())) == n.Int64() {
		return int(n.Int64()), nil
	}
	return n, nil
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
