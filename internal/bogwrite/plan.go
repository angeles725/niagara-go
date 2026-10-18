package bogwrite

import (
	"errors"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
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
	if e := pycompat.FindUTF8Error(data); e != nil {
		return "", raise("UnicodeDecodeError", "%s", e)
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n"), nil
}
