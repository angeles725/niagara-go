package pycompat

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Percent is `format % params` with a mapping, for the subset argparse and the tools use:
// %(key)s, %(key)r, %(key)d / %(key)i and %%. Flags, width and precision are not supported
// and are an error, like the cases where Python raises (missing key, a bare %s, a bad
// conversion, a truncated directive).
func Percent(format string, params map[string]any) (string, error) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(format) {
			return "", errors.New("incomplete format")
		}
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}
		if format[i] != '(' {
			return "", errors.New("format requires a mapping")
		}
		end := strings.IndexByte(format[i:], ')')
		if end < 0 {
			return "", errors.New("incomplete format key")
		}
		key := format[i+1 : i+end]
		i += end + 1
		if i >= len(format) {
			return "", errors.New("incomplete format")
		}
		v, ok := params[key]
		if !ok {
			return "", errors.New("KeyError: " + key)
		}
		switch format[i] {
		case 's':
			b.WriteString(Str(v))
		case 'r':
			b.WriteString(Repr(v))
		case 'd', 'i':
			s, err := percentInt(v)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		default:
			return "", errors.New("unsupported format character " + strconv.QuoteRune(rune(format[i])))
		}
	}
	return b.String(), nil
}

func percentInt(v any) (string, error) {
	switch n := v.(type) {
	case bool:
		if n {
			return "1", nil
		}
		return "0", nil
	case int:
		return strconv.Itoa(n), nil
	case int64:
		return strconv.FormatInt(n, 10), nil
	case *big.Int:
		return n.String(), nil
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return "", errors.New("cannot convert float to integer")
		}
		s, _ := IntOfFloat(n)
		return s, nil
	}
	return "", errors.New("%d format: a real number is required")
}
