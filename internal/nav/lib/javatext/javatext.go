// Package javatext is the shared part of module-navigator
// module_nav_lib/java_text.py and the plain-text matchers its ports share:
// comment spans, the declaration header, the declaration keywords and re's
// \w / \s tests at a byte offset. Helpers only one port uses stay with it
// (call_args, brace_blocks and the like in internal/nav/lib/license).
//
// It is a leaf: it imports nothing from internal/nav, so core, the library
// packages and the builders (internal/navbuild) can all use it. Offsets are
// byte offsets into UTF-8 text: the Python code compares only ASCII
// characters, so stepping bytes instead of code points finds the same
// positions; \w and \s are Python's Unicode classes (pycompat.IsWordRune,
// textfold.IsSpace).
package javatext

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/textfold"
)

// Span is a [Start, End) byte range.
type Span struct{ Start, End int }

// CommentSpans is java_text.comment_spans: every // and /* */ comment,
// skipping string and char literals (which end at a newline).
func CommentSpans(text string) []Span {
	var spans []Span
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
			spans = append(spans, Span{i, j})
			i = j
		case strings.HasPrefix(text[i:], "/*"):
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				j = n
			} else {
				j += i + 4
			}
			spans = append(spans, Span{i, j})
			i = j
		default:
			i++
		}
	}
	return spans
}

// StripComments is java_text.strip_comments: text with every comment
// (CommentSpans) replaced by one space.
func StripComments(text string) string {
	spans := CommentSpans(text)
	if len(spans) == 0 {
		return text
	}
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(text[last:s.Start])
		b.WriteByte(' ')
		last = s.End
	}
	b.WriteString(text[last:])
	return b.String()
}

// MaxHeaderLines is java_text.MAX_HEADER_LINES: a declaration header spans a
// handful of lines; the cap only stops a malformed file from joining the rest
// of itself.
const MaxHeaderLines = 40

// DeclarationHeader is java_text.declaration_header(lines, index, start): the
// declaration in lines[index] from byte start up to its opening brace, joined
// across at most MaxHeaderLines lines, comments removed and whitespace
// collapsed. lines keep their "\n" (readlines()).
func DeclarationHeader(lines []string, index, start int) string {
	raw := lines[index][start:]
	end := min(len(lines), index+MaxHeaderLines)
	var text string
	for j := index + 1; ; j++ {
		text = StripComments(raw)
		if brace := strings.IndexByte(text, '{'); brace >= 0 {
			text = text[:brace]
			break
		}
		if j >= end {
			break
		}
		raw += "\n" + lines[j]
	}
	return strings.Join(textfold.Fields(text), " ")
}

// Modifiers and Kinds are the alternatives of the type-declaration regexes
// (corpus.py DECL_RE, recall.py _TOP_LEVEL_DECL), in their order.
var (
	Modifiers = []string{"public", "protected", "private", "static", "abstract", "final",
		"strictfp", "sealed", "non-sealed"}
	Kinds = []string{"class", "interface", "@interface", "enum", "record"}
)
