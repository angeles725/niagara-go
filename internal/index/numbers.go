package index

import (
	"encoding/json"
	"strconv"
	"strings"
)

// IsInt reports whether a decoded number was an integer literal (Python int),
// not a float literal (Python float): no fraction, no exponent, not NaN or
// an infinity.
func IsInt(n json.Number) bool {
	return !strings.ContainsAny(string(n), ".eEIN")
}

// Int returns v as an int64 when it is an integer literal that fits.
func Int(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok || !IsInt(n) {
		return 0, false
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	return i, err == nil
}

// Float returns v as a float64 when it is any number (Python float(x) of an
// int or a float). An out-of-range literal is ±Inf, like Python's float().
func Float(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		if ne, isNum := err.(*strconv.NumError); !isNum || ne.Err != strconv.ErrRange {
			return 0, false
		}
	}
	return f, true
}
