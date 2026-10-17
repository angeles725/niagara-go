package bogwrite

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// decodePlan is json.loads(text) (internal/pyjson, CPython's C scanner) with the exception
// name the traceback's last line carries.
func decodePlan(text string) (pyjson.Value, error) {
	v, err := pyjson.Loads([]rune(text))
	if err == nil {
		return v, nil
	}
	var digits *pyjson.IntDigitsError
	var deep *pyjson.RecursionError
	switch {
	case errors.As(err, &digits):
		return v, raise("ValueError", "%s", err)
	case errors.As(err, &deep):
		return v, raise("RecursionError", "%s", err)
	}
	return v, raise("json.decoder.JSONDecodeError", "%s", err)
}

// readPlanText is open(path, encoding="utf-8").read(): strict UTF-8 with CPython's
// UnicodeDecodeError text, then universal newlines (CRLF and CR become LF).
func readPlanText(data []byte) (string, error) {
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || size > 1 {
			i += size
			continue
		}
		return "", utf8Error(data, i)
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n"), nil
}

// utf8Error describes the bad sequence at i like CPython's UTF-8 decoder.
func utf8Error(data []byte, i int) error {
	c := data[i]
	need, lo, hi := 0, byte(0x80), byte(0xBF)
	switch {
	case 0xC2 <= c && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case 0xE1 <= c && c <= 0xEC || c == 0xEE || c == 0xEF:
		need = 2
	case c == 0xED:
		need, hi = 2, 0x9F
	case c == 0xF0:
		need, lo = 3, 0x90
	case 0xF1 <= c && c <= 0xF3:
		need = 3
	case c == 0xF4:
		need, hi = 3, 0x8F
	default:
		return raise("UnicodeDecodeError", "'utf-8' codec can't decode byte 0x%02x in position %d: invalid start byte", c, i)
	}
	j := i + 1
	for k := 0; k < need; k, j = k+1, j+1 {
		if j >= len(data) {
			if j-i == 1 {
				return raise("UnicodeDecodeError", "'utf-8' codec can't decode byte 0x%02x in position %d: unexpected end of data", c, i)
			}
			return raise("UnicodeDecodeError", "'utf-8' codec can't decode bytes in position %d-%d: unexpected end of data", i, j-1)
		}
		b := data[j]
		if k == 0 && (b < lo || b > hi) || k > 0 && (b < 0x80 || b > 0xBF) {
			break
		}
	}
	// j is the bad byte: CPython names the valid prefix before it
	if j-i == 1 {
		return raise("UnicodeDecodeError", "'utf-8' codec can't decode byte 0x%02x in position %d: invalid continuation byte", c, i)
	}
	return raise("UnicodeDecodeError", "'utf-8' codec can't decode bytes in position %d-%d: invalid continuation byte", i, j-1)
}
