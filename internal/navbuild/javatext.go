package navbuild

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The declaration parsing of build_class_index.py (java_text.py itself is
// internal/nav/lib/javatext). Offsets are byte offsets: the Python code
// compares only ASCII characters, so stepping bytes instead of code points
// finds the same positions.

// spanFunc advances over the runes of s from i that satisfy ok.
func spanFunc(s string, i int, ok func(rune) bool) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !ok(r) {
			break
		}
		i += n
	}
	return i
}

// splitTypeList is build_class_index.split_type_list: comma-separated types,
// commas inside <> nesting kept, each stripped, empty ones dropped.
func splitTypeList(s string) []string {
	parts := []string{}
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				if t := pycompat.Strip(s[start:i]); t != "" {
					parts = append(parts, t)
				}
				start = i + 1
			}
		}
	}
	if t := pycompat.Strip(s[start:]); t != "" {
		parts = append(parts, t)
	}
	return parts
}

// optString is a str or None.
type optString struct {
	s     string
	valid bool
}

// parseExtendsImplements is build_class_index.parse_extends_implements: the
// extends type (None when absent or empty after trimming commas; "" in the
// odd case of implements before extends, as Python's reversed slice gives)
// and the implements list of a declaration header.
func parseExtendsImplements(text string) (optString, []string) {
	if brace := strings.IndexByte(text, '{'); brace >= 0 {
		text = text[:brace]
	}
	text = pycompat.Strip(text)
	if strings.HasPrefix(text, "<") { // skip leading type parameters <T, U extends ...>
		depth := 0
		for i := 0; i < len(text); i++ {
			switch text[i] {
			case '<':
				depth++
			case '>':
				depth--
			}
			if depth == 0 {
				text = pycompat.Strip(text[i+1:])
				break
			}
		}
	}
	var extends optString
	implements := []string{}
	_, ee, hasExt := keywordSpace(text, "extends")
	is, ie, hasImpl := keywordSpace(text, "implements")
	switch {
	case hasExt && hasImpl:
		if is >= ee {
			extends = optString{pycompat.Strip(text[ee:is]), true}
		} else {
			extends = optString{"", true}
		}
		implements = splitTypeList(text[ie:])
	case hasExt:
		extends = optString{pycompat.Strip(text[ee:]), true}
	case hasImpl:
		implements = splitTypeList(text[ie:])
	}
	if extends.s != "" {
		extends.s = pycompat.Strip(strings.TrimRight(extends.s, ","))
		if extends.s == "" {
			extends.valid = false
		}
	}
	return extends, implements
}

// keywordSpace is re.search(r'\b<kw>\b\s+', text): the span of the first match.
func keywordSpace(text, kw string) (start, end int, ok bool) {
	for p := 0; p < len(text); {
		k := strings.Index(text[p:], kw)
		if k < 0 {
			return 0, 0, false
		}
		a := p + k
		prev, _ := utf8.DecodeLastRuneInString(text[:a])
		next, _ := utf8.DecodeRuneInString(text[a+len(kw):])
		before := a == 0 || !pycompat.IsWordRune(prev)
		after := a+len(kw) == len(text) || !pycompat.IsWordRune(next)
		if before && after {
			if e := spanFunc(text, a+len(kw), pycompat.IsSpace); e > a+len(kw) {
				return a, e, true
			}
		}
		p = a + 1
	}
	return 0, 0, false
}
