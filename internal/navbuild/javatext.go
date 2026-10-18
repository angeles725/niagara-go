package navbuild

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Plain-text helpers for Java sources (module_nav_lib/java_text.py and the
// declaration parsing of build_class_index.py). Offsets are byte offsets: the
// Python code compares only ASCII characters, so stepping bytes instead of
// code points finds the same positions.

// commentSpans is java_text.comment_spans: the [start, end) byte range of
// every // and /* */ comment, skipping string and char literals (which end at
// a newline). (A twin lives unexported in nav/license; see docs/navbuild.md.)
func commentSpans(text string) [][2]int {
	var spans [][2]int
	n := len(text)
	for i := 0; i < n; {
		c := text[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && text[j] != c && text[j] != '\n' {
				if text[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			i = j + 1
		case strings.HasPrefix(text[i:], "//"):
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				j = n
			} else {
				j += i
			}
			spans = append(spans, [2]int{i, j})
			i = j
		case strings.HasPrefix(text[i:], "/*"):
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				j = n
			} else {
				j += i + 4
			}
			spans = append(spans, [2]int{i, j})
			i = j
		default:
			i++
		}
	}
	return spans
}

// stripComments is java_text.strip_comments: text with every comment replaced
// by one space.
func stripComments(text string) string {
	spans := commentSpans(text)
	if len(spans) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(text[last:s[0]])
		b.WriteByte(' ')
		last = s[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// maxHeaderLines is java_text.MAX_HEADER_LINES: a declaration header spans a
// handful of lines; the cap only stops a malformed file from joining the rest
// of itself.
const maxHeaderLines = 40

// declarationHeader is java_text.declaration_header(lines, index, start): the
// declaration in lines[index] from byte start up to its opening brace, joined
// across at most maxHeaderLines lines, comments removed and whitespace
// collapsed. lines keep their "\n" (readlines()).
func declarationHeader(lines []string, index, start int) string {
	raw := lines[index][start:]
	end := min(len(lines), index+maxHeaderLines)
	var text string
	for j := index + 1; ; j++ {
		text = stripComments(raw)
		if brace := strings.IndexByte(text, '{'); brace >= 0 {
			text = text[:brace]
			break
		}
		if j >= end {
			break
		}
		raw += "\n" + lines[j]
	}
	return strings.Join(pycompat.Fields(text), " ")
}

// declEnd is m.end() of the corpus.DECL_RE match d of line, which
// core.MatchDecl does not report: the end of the declared name. The name
// follows Modifiers+Kind and at least one space; a candidate position is
// confirmed by matching the line cut there (plus " (" for the record
// lookahead) to the same declaration, which rules out the same text inside
// an annotation argument. -1 when no candidate confirms (never for a line
// MatchDecl matched).
func declEnd(line string, d core.Decl) int {
	head := d.Modifiers + d.Kind
	for p := len(d.Indent); p < len(line); {
		k := strings.Index(line[p:], head)
		if k < 0 {
			break
		}
		a := p + k + len(head)
		e := spanFunc(line, a, pycompat.IsSpace)
		if e > a && strings.HasPrefix(line[e:], d.Name) {
			end := e + len(d.Name)
			if r, _ := utf8.DecodeRuneInString(line[end:]); end == len(line) || !(r == '$' || pycompat.IsWordRune(r)) {
				if got, ok := core.MatchDecl(line[:end] + " ("); ok && got == d {
					return end
				}
			}
		}
		p += k + 1
	}
	return -1
}

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
