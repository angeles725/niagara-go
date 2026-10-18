package core

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/nav/lib/javatext"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Hand scanners for the declaration regexes of corpus.py. Python's versions use
// lookarounds (absent from Go's regexp); each pattern below is deterministic
// (no alternative can succeed after a greedy part fails), so one left-to-right
// pass gives the same match. \s and \w are Python's Unicode classes.

// Decl is a DECL_RE match: one type declaration line.
type Decl struct {
	Indent    string // group 1
	Modifiers string // group 2, with its trailing whitespace ("public static ")
	Kind      string // class, interface, @interface, enum or record
	Name      string
	End       int // m.end() as a byte offset: just after Name, where the header continues
}

func runeAt(s string, i int) rune {
	if i >= len(s) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return r
}

// span advances over the runes of s from i that satisfy ok.
func span(s string, i int, ok func(rune) bool) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !ok(r) {
			break
		}
		i += n
	}
	return i
}

func isWordOrDollar(r rune) bool { return r == '$' || pycompat.IsWordRune(r) }

func isWordOrDot(r rune) bool { return r == '.' || pycompat.IsWordRune(r) }

// wordEnds is \b after a word ending at i.
func wordEnds(s string, i int) bool { return !pycompat.IsWordRune(runeAt(s, i)) }

// MatchDecl is corpus.DECL_RE.match(line):
//
//	^(\s*)(?:@(?!interface\b)[\w.]+(?:\([^)]*\))?\s+)*((?:MODIFIER\s+)*)
//	(class|interface|@interface|enum|record(?=\s+\w+\s*[(<]))\s+([\w$]+)
func MatchDecl(line string) (Decl, bool) {
	i := span(line, 0, pycompat.IsSpace)
	d := Decl{Indent: line[:i]}
	for {
		j, ok := annotation(line, i)
		if !ok {
			break
		}
		i = j
	}
	start := i
	for {
		j, ok := modifier(line, i)
		if !ok {
			break
		}
		i = j
	}
	d.Modifiers = line[start:i]
	for _, k := range javatext.Kinds {
		if !strings.HasPrefix(line[i:], k) || (k == "record" && !recordHeader(line, i+len(k))) {
			continue
		}
		j := span(line, i+len(k), pycompat.IsSpace)
		if j == i+len(k) {
			return Decl{}, false
		}
		end := span(line, j, isWordOrDollar)
		if end == j {
			return Decl{}, false
		}
		d.Kind, d.Name, d.End = k, line[j:end], end
		return d, true
	}
	return Decl{}, false
}

// annotation matches @(?!interface\b)[\w.]+(?:\([^)]*\))?\s+ at i.
func annotation(s string, i int) (int, bool) {
	if runeAt(s, i) != '@' {
		return i, false
	}
	if strings.HasPrefix(s[i+1:], "interface") && wordEnds(s, i+1+len("interface")) {
		return i, false
	}
	j := span(s, i+1, isWordOrDot)
	if j == i+1 {
		return i, false
	}
	if runeAt(s, j) == '(' {
		if k := strings.IndexByte(s[j:], ')'); k >= 0 {
			if e := span(s, j+k+1, pycompat.IsSpace); e > j+k+1 {
				return e, true
			}
		}
	}
	if e := span(s, j, pycompat.IsSpace); e > j {
		return e, true
	}
	return i, false
}

// modifier matches one (?:public|...|non-sealed)\s+ at i.
func modifier(s string, i int) (int, bool) {
	for _, m := range javatext.Modifiers {
		if strings.HasPrefix(s[i:], m) {
			if e := span(s, i+len(m), pycompat.IsSpace); e > i+len(m) {
				return e, true
			}
			return i, false
		}
	}
	return i, false
}

// recordHeader is the lookahead (?=\s+\w+\s*[(<]) at i.
func recordHeader(s string, i int) bool {
	j := span(s, i, pycompat.IsSpace)
	if j == i {
		return false
	}
	k := span(s, j, pycompat.IsWordRune)
	if k == j {
		return false
	}
	r := runeAt(s, span(s, k, pycompat.IsSpace))
	return r == '(' || r == '<'
}

// TypeDeclName is corpus.type_decl_name: the type name of the first match of
// \b(?:class|interface|enum)\s+([\w$]+)|\brecord\s+([\w$]+)(?=\s*[(<]) in line.
func TypeDeclName(line string) (string, bool) {
	for p := 0; p < len(line); {
		prev, _ := utf8.DecodeLastRuneInString(line[:p])
		if p == 0 || !pycompat.IsWordRune(prev) {
			for _, k := range []string{"class", "interface", "enum"} {
				if name, _, ok := keywordName(line, p, k); ok {
					return name, true
				}
			}
			if name, end, ok := keywordName(line, p, "record"); ok {
				if c := runeAt(line, span(line, end, pycompat.IsSpace)); c == '(' || c == '<' {
					return name, true
				}
			}
		}
		_, n := utf8.DecodeRuneInString(line[p:])
		p += n
	}
	return "", false
}

// keywordName matches keyword\s+([\w$]+) at p: the name and where it ends.
func keywordName(s string, p int, keyword string) (string, int, bool) {
	if !strings.HasPrefix(s[p:], keyword) {
		return "", p, false
	}
	j := span(s, p+len(keyword), pycompat.IsSpace)
	if j == p+len(keyword) {
		return "", p, false
	}
	end := span(s, j, isWordOrDollar)
	if end == j {
		return "", p, false
	}
	return s[j:end], end, true
}

// StartsWithAnnotation is corpus.ANNOTATION_LINE_RE.match(s): s starts with
// an annotation, not an @interface declaration.
func StartsWithAnnotation(s string) bool {
	return strings.HasPrefix(s, "@") &&
		!(strings.HasPrefix(s[1:], "interface") && wordEnds(s, 1+len("interface")))
}

// PackageName is corpus.PACKAGE_RE.match(line): ^package\s+([\w.]+)\s*;
func PackageName(line string) (string, bool) {
	if !strings.HasPrefix(line, "package") {
		return "", false
	}
	j := span(line, len("package"), pycompat.IsSpace)
	if j == len("package") {
		return "", false
	}
	end := span(line, j, isWordOrDot)
	if end == j || runeAt(line, span(line, end, pycompat.IsSpace)) != ';' {
		return "", false
	}
	return line[j:end], true
}
