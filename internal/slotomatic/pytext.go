package slotomatic

import (
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// ReadSource reads a source file the way open(path, encoding="utf-8", errors="replace").read()
// does: undecodable bytes become U+FFFD (pycompat.DecodeReplace) and "\r\n" and "\r" become
// "\n" (universal newlines).
func ReadSource(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return UniversalNewlines(pycompat.DecodeReplace(raw)), nil
}

// UniversalNewlines turns "\r\n" and a lone "\r" into "\n", as Python's text mode does.
func UniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
