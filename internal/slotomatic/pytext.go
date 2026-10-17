package slotomatic

import (
	"os"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// ReadSource reads a source file the way open(path, encoding="utf-8", errors="replace").read()
// does: undecodable bytes become U+FFFD (pycompat.DecodeReplace) and "\r\n" and "\r" become
// "\n" (pycompat.UniversalNewlines).
func ReadSource(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return pycompat.UniversalNewlines(pycompat.DecodeReplace(raw)), nil
}
