package pycompat

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrSyntax is the ValueError of int() or float(): the text is not a literal.
	ErrSyntax = errors.New("invalid literal")
	// ErrRange reports an int() literal that is valid but does not fit in an int64.
	ErrRange = errors.New("int out of int64 range")
)

// asciiDigits is _PyUnicode_TransformDecimalAndSpaceToASCII after strip(): decimal digits
// become ASCII; ok is false when a non-ASCII character is neither a digit nor kept.
func asciiDigits(s string) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case decimalValue(r) >= 0:
			b.WriteByte(byte('0' + decimalValue(r)))
		default:
			return "", false
		}
	}
	return b.String(), true
}

// ParseBigInt is int(text): surrounding whitespace, an optional sign, decimal digits (any
// Unicode decimal) with single underscores between them, any magnitude.
func ParseBigInt(text string) (*big.Int, error) {
	s, ok := asciiDigits(Strip(text))
	if !ok {
		return nil, ErrSyntax
	}
	sign := ""
	if s != "" && (s[0] == '+' || s[0] == '-') {
		sign, s = s[:1], s[1:]
	}
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' || strings.Contains(s, "__") {
		return nil, ErrSyntax
	}
	digits := strings.ReplaceAll(s, "_", "")
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return nil, ErrSyntax
		}
	}
	n, _ := new(big.Int).SetString(sign+digits, 10)
	return n, nil
}

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

var floatSyntax = regexp.MustCompile(`^[+-]?(?:` +
	`(?:[0-9](?:_?[0-9])*(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)(?:[eE][+-]?[0-9](?:_?[0-9])*)?` +
	`|(?i:inf|infinity|nan))$`)

// ParseFloat is float(text): surrounding whitespace, Unicode decimal digits, underscores
// between digits, inf, infinity and nan in any case and with a sign, no hex; overflow
// gives ±inf like Python.
func ParseFloat(text string) (float64, error) {
	s, ok := asciiDigits(Strip(text))
	if !ok || !floatSyntax.MatchString(s) {
		return 0, ErrSyntax
	}
	if strings.EqualFold(strings.TrimLeft(s, "+-"), "nan") {
		return math.NaN(), nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, ErrSyntax
	}
	return f, nil
}
