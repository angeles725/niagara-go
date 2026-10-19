// Package codesearch is the search logic of the module-nav `grep` and `token`
// commands (module_nav_lib/grep_search.py and tokens.py) as a library: it
// prints nothing, returns typed rows, does not depend on the nav runtime or
// registry, and exports the formatters that yield each row's exact CLI line,
// so the command and nsearch print the same text (decision D29).
package codesearch

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// ReadLines is open(path, encoding="utf-8", errors="replace").readlines():
// invalid bytes become U+FFFD, "\r\n" and a lone "\r" become "\n" (universal
// newlines), and every line but possibly the last keeps its "\n".
func ReadLines(path string) ([]string, error) {
	text, err := pycompat.ReadTextReplace(path)
	if err != nil {
		return nil, err
	}
	return pycompat.Lines(text), nil
}

// RStrip is str.rstrip() with no argument (Python's Unicode whitespace).
func RStrip(s string) string { return strings.TrimRightFunc(s, textfold.IsSpace) }

// Ellipsis is the display cut of a source line or a name: s when it has at
// most max code points, else its first max-3 code points and "...".
func Ellipsis(s string, max int) string {
	if len(s) <= max || utf8.RuneCountInString(s) <= max {
		return s
	}
	return pycompat.Cut(s, max-3) + "..."
}
