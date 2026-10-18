package inventory

import (
	"os"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The declaration scan of the recall oracle (recall.py _TOP_LEVEL_DECL,
// _declarations, _header_keywords and java_text.declaration_header).

// recallModifiers and recallKinds are the alternatives of _TOP_LEVEL_DECL.
var (
	recallModifiers = []string{"public", "protected", "private", "static", "abstract", "final", "strictfp",
		"sealed", "non-sealed"}
	recallKinds = []string{"class", "interface", "@interface", "enum", "record"}
)

// topLevelDecl is _TOP_LEVEL_DECL.match(line):
//
//	^(?:@\w[\w.]*(?:\([^)]*\))?\s+)*(?:(?:MODIFIER)\s+)*
//	(?:class|interface|@interface|enum|record)\s+([A-Za-z_$][\w$]*)
//
// the declared name and the end of the match; the annotation and modifier
// repetitions are tried greedily, backing off as the regex does (an
// "@interface" can be read as an annotation).
func topLevelDecl(line string) (name string, end int, ok bool) {
	ann := []int{0}
	for {
		next, ok := recallAnnotation(line, ann[len(ann)-1])
		if !ok {
			break
		}
		ann = append(ann, next)
	}
	for a := len(ann) - 1; a >= 0; a-- {
		mods := []int{ann[a]}
		for {
			next, ok := recallModifier(line, mods[len(mods)-1])
			if !ok {
				break
			}
			mods = append(mods, next)
		}
		for m := len(mods) - 1; m >= 0; m-- {
			if name, end, ok := recallKindName(line, mods[m]); ok {
				return name, end, true
			}
		}
	}
	return "", 0, false
}

// spaces is \s+ from i: the end, ok=false when there is none.
func spaces(s string, i int) (int, bool) {
	j := i
	for j < len(s) {
		r, n := utf8.DecodeRuneInString(s[j:])
		if !pycompat.IsSpace(r) {
			break
		}
		j += n
	}
	return j, j > i
}

func runeIs(s string, i int, ok func(rune) bool) (int, bool) {
	if i >= len(s) {
		return 0, false
	}
	r, n := utf8.DecodeRuneInString(s[i:])
	return n, ok(r)
}

// recallAnnotation is @\w[\w.]*(?:\([^)]*\))?\s+ at i.
func recallAnnotation(s string, i int) (int, bool) {
	if i >= len(s) || s[i] != '@' {
		return 0, false
	}
	n, ok := runeIs(s, i+1, pycompat.IsWordRune)
	if !ok {
		return 0, false
	}
	j := i + 1 + n
	for {
		n, ok := runeIs(s, j, func(r rune) bool { return r == '.' || pycompat.IsWordRune(r) })
		if !ok {
			break
		}
		j += n
	}
	if j < len(s) && s[j] == '(' {
		if k := strings.IndexByte(s[j+1:], ')'); k >= 0 {
			if end, ok := spaces(s, j+1+k+1); ok {
				return end, true
			}
		}
	}
	return spaces(s, j)
}

// recallModifier is (?:MODIFIER)\s+ at i, the alternatives in order.
func recallModifier(s string, i int) (int, bool) {
	for _, m := range recallModifiers {
		if strings.HasPrefix(s[i:], m) {
			if end, ok := spaces(s, i+len(m)); ok {
				return end, true
			}
		}
	}
	return 0, false
}

// recallKindName is (?:KIND)\s+([A-Za-z_$][\w$]*) at i.
func recallKindName(s string, i int) (string, int, bool) {
	for _, k := range recallKinds {
		if !strings.HasPrefix(s[i:], k) {
			continue
		}
		j, ok := spaces(s, i+len(k))
		if !ok || j >= len(s) {
			continue
		}
		c := s[j]
		if !(c == '_' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			continue
		}
		end := j + 1
		for {
			n, ok := runeIs(s, end, func(r rune) bool { return r == '$' || pycompat.IsWordRune(r) })
			if !ok {
				break
			}
			end += n
		}
		return s[j:end], end, true
	}
	return "", 0, false
}

// declarations is _declarations: every top-level declaration of a file with
// the extends/implements keywords its header names.
func declarations(path string) ([]string, [][]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := pycompat.Lines(pycompat.DecodeReplace(b))
	var names []string
	var keywords [][]string
	for i, line := range lines {
		if name, end, ok := topLevelDecl(line); ok {
			names = append(names, name)
			keywords = append(keywords, headerKeywords(lines, i, end))
		}
	}
	return names, keywords, nil
}

// maxHeaderLines is java_text.MAX_HEADER_LINES.
const maxHeaderLines = 40

// headerKeywords is _header_keywords: the sorted {extends, implements} named
// in the declaration header (java_text.declaration_header), type arguments
// dropped.
func headerKeywords(lines []string, index, start int) []string {
	raw := lines[index][start:]
	end := min(len(lines), index+maxHeaderLines)
	j := index + 1
	var text string
	for {
		text = stripComments(raw)
		if b := strings.IndexByte(text, '{'); b >= 0 {
			text = text[:b]
			break
		}
		if j >= end {
			break
		}
		raw += "\n" + lines[j]
		j++
	}
	text = strings.Join(pycompat.Fields(text), " ")
	depth := 0
	var outside strings.Builder
	for _, c := range text {
		switch c {
		case '<':
			depth++
		case '>':
			depth--
		}
		if depth == 0 && c != '>' {
			outside.WriteRune(c)
		} else {
			outside.WriteByte(' ')
		}
	}
	o := outside.String()
	var out []string
	for _, kw := range []string{"extends", "implements"} {
		for i := 0; ; {
			k := strings.Index(o[i:], kw)
			if k < 0 {
				break
			}
			p := i + k
			if !wordAt(o, p-1, true) && !wordAt(o, p+len(kw), false) {
				out = append(out, kw)
				break
			}
			i = p + 1
		}
	}
	return out
}

// wordAt tells whether the rune ending (before) or starting (!before) at
// byte i of s is a word character.
func wordAt(s string, i int, before bool) bool {
	if before {
		if i < 0 {
			return false
		}
		r, _ := utf8.DecodeLastRuneInString(s[:i+1])
		return pycompat.IsWordRune(r)
	}
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return pycompat.IsWordRune(r)
}

// stripComments is java_text.strip_comments: every comment replaced by one
// space, string and char literals (which end at a newline) skipped.
func stripComments(text string) string {
	var b strings.Builder
	last, n := 0, len(text)
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
		case strings.HasPrefix(text[i:], "//"), strings.HasPrefix(text[i:], "/*"):
			j := n
			if c2 := text[i+1]; c2 == '/' {
				if k := strings.IndexByte(text[i:], '\n'); k >= 0 {
					j = i + k
				}
			} else if k := strings.Index(text[i+2:], "*/"); k >= 0 {
				j = i + 2 + k + 2
			}
			b.WriteString(text[last:i])
			b.WriteByte(' ')
			last, i = j, j
		default:
			i++
		}
	}
	b.WriteString(text[min(last, n):])
	return b.String()
}
