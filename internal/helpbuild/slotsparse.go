package helpbuild

import (
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The annotation scanner of slots_indexer.py: annotation_bodies and the
// helpers it shares with _split_top_level and _parse_annotation_body. The
// text is code points (Python str indexing); an index may run past the
// end, as Python's does, and every slice is clamped like a Python slice.

// strKV is one key/value pair of a dict of strings.
type strKV struct{ key, val string }

// strDict is a dict of strings in insertion order: setting a key again
// keeps its first position and takes the last value, as in Python.
type strDict []strKV

func (d *strDict) set(key, val string) {
	for i := range *d {
		if (*d)[i].key == key {
			(*d)[i].val = val
			return
		}
	}
	*d = append(*d, strKV{key, val})
}

func (d strDict) get(key string) string {
	for _, kv := range d {
		if kv.key == key {
			return kv.val
		}
	}
	return ""
}

// runeSlice is text[from:to] with Python's clamping.
func runeSlice(text []rune, from, to int) []rune {
	to = min(to, len(text))
	if from >= to {
		return nil
	}
	return text[from:to]
}

// skipLiteralOrComment is _skip_literal_or_comment: the index just past
// the string/char literal or comment starting at text[i] (len(text) or
// beyond for an unclosed one), -1 when none starts there.
func skipLiteralOrComment(text []rune, i int) int {
	c := text[i]
	switch {
	case c == '"' || c == '\'':
		i++
		for i < len(text) && text[i] != c {
			if text[i] == '\\' {
				i += 2
			} else {
				i++
			}
		}
		return i + 1
	case c == '/' && i+1 < len(text) && text[i+1] == '/':
		for j := i + 2; j < len(text); j++ {
			if text[j] == '\n' {
				return j
			}
		}
		return len(text)
	case c == '/' && i+1 < len(text) && text[i+1] == '*':
		for j := i + 2; j+1 < len(text); j++ {
			if text[j] == '*' && text[j+1] == '/' {
				return j + 2
			}
		}
		return len(text)
	}
	return -1
}

// closers are the brackets _scan counts.
var closers = map[rune]rune{'(': ')', '{': '}', '[': ']'}

// scanGroup is _scan: the index just past the close rune that ends the
// group opened before text[i], skipping nested brackets, literals and
// comments; a different closing bracket at depth 0 stops at its own index.
func scanGroup(text []rune, i int, close rune) int {
	depth := 0
	for i < len(text) {
		if skip := skipLiteralOrComment(text, i); skip >= 0 {
			i = skip
			continue
		}
		c := text[i]
		if _, open := closers[c]; open {
			depth++
		} else if c == ')' || c == '}' || c == ']' {
			if depth == 0 {
				if c == close {
					return i + 1
				}
				return i
			}
			depth--
		}
		i++
	}
	return i
}

// stripComments is _strip_comments: every comment outside a literal
// replaced by one space.
func stripComments(text []rune) []rune {
	out := make([]rune, 0, len(text))
	for i := 0; i < len(text); {
		skip := skipLiteralOrComment(text, i)
		if skip < 0 {
			out = append(out, text[i])
			i++
			continue
		}
		if text[i] == '/' {
			out = append(out, ' ')
		} else {
			out = append(out, runeSlice(text, i, skip)...)
		}
		i = skip
	}
	return out
}

// annotationRes are the @Name\s*\( patterns annotation_bodies searches.
var annotationRes = map[string]*pyre.Regexp{}

func init() {
	for _, kind := range []string{"NiagaraProperty", "NiagaraAction", "NiagaraTopic"} {
		annotationRes[kind] = pyre.MustCompile(`@`+kind+`\s*\(`, 0)
	}
}

// annotationBodies is annotation_bodies: the text between the outer
// parentheses of every @kind(...) annotation, comments removed.
func annotationBodies(content []rune, kind string) [][]rune {
	var out [][]rune
	for _, m := range annotationRes[kind].FindIter(content) {
		start := m.End()
		end := scanGroup(content, start, ')')
		out = append(out, stripComments(runeSlice(content, start, end-1)))
	}
	return out
}

// splitTopLevel is _split_top_level: the body cut at the commas outside
// brackets and literals.
func splitTopLevel(body []rune) [][]rune {
	var parts [][]rune
	start := 0
	for i := 0; i < len(body); {
		if skip := skipLiteralOrComment(body, i); skip >= 0 {
			i = skip
			continue
		}
		c := body[i]
		if close, open := closers[c]; open {
			i = scanGroup(body, i+1, close)
			continue
		}
		if c == ',' {
			parts = append(parts, body[start:i])
			start = i + 1
		}
		i++
	}
	return append(parts, runeSlice(body, start, len(body)))
}

// isAnnotationKey is re.match(r"^\s*\w+\s*$", key).
func isAnnotationKey(key []rune) bool {
	i, j := 0, len(key)
	for i < j && pyre.IsSpace(key[i]) {
		i++
	}
	for j > i && pyre.IsSpace(key[j-1]) {
		j--
	}
	if i == j {
		return false
	}
	for _, r := range key[i:j] {
		if !pyre.IsWord(r) {
			return false
		}
	}
	return true
}

// parseAnnotationBody is _parse_annotation_body: the top-level key = value
// pairs; a value that is one string literal loses its quotes, any other
// expression is kept whole with its whitespace collapsed.
func parseAnnotationBody(body []rune) strDict {
	var result strDict
	for _, part := range splitTopLevel(body) {
		eq := -1
		for i, r := range part {
			if r == '=' {
				eq = i
				break
			}
		}
		if eq < 0 || !isAnnotationKey(part[:eq]) {
			continue
		}
		value := []rune(collapseSpace(pycompat.Strip(string(part[eq+1:]))))
		if n := len(value); n >= 2 && value[0] == '"' && value[n-1] == '"' && skipLiteralOrComment(value, 0) == n {
			value = value[1 : n-1]
		}
		result.set(pycompat.Strip(string(part[:eq])), string(value))
	}
	return result
}
