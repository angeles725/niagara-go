package greenmaxmatrix

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// pyRe rewrites every \s of a pattern as Python's Unicode whitespace class (pycompat.RE2Space)
// and a trailing `$` as Python's (end, or before a final newline).
func pyRe(pattern string) string {
	pattern = pycompat.RE2Space(pattern)
	if strings.HasSuffix(pattern, "$") {
		pattern = strings.TrimSuffix(pattern, "$") + `\n?$`
	}
	return pattern
}
