// Package slotomatic parses the Slotomatic slot declarations of a Niagara 4 Java source file:
//
//	public static final Property in8 = newProperty(Flags.READONLY, new BStatusNumeric(...), null);
//	public static final Action   set = newAction(Flags.OPERATOR, BDouble.DEFAULT, null);
//	public static final Topic    ev  = newTopic(Flags.SUMMARY, null);
//
// It is the Go port of parse_source and decode_flags of niagara-research tools/n4-type-catalog.py,
// shared by n4-type-catalog (the catalog) and bog-nav (effective slot defaults). It reads original
// javadoc source, CFR/Vineflower output (numeric flags such as `(int)1026`) and Procyon output
// (`public static final Property x;` assigned later in a `static {}` block). Expressions stay
// strings, never evaluated. Character classes follow Python's re on str patterns: \s is
// str.isspace(), \w is str.isalnum() or '_', \d is a Unicode decimal digit.
package slotomatic

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Decl is one declared slot.
type Decl struct {
	Name        string
	Flags       int64
	FlagLetters string   // the letters of the known flag bits, in bit order
	Args        []string // every call argument, cleaned
	Default     string   // the default value expression ("" when the call has none)
	Facets      string   // the facets expression ("null" when absent)
}

// Type is the declarations of one source file's public class.
type Type struct {
	Package    string
	Class      string
	Extends    string
	Properties []Decl
	Actions    []Decl
	Topics     []Decl
}

// Python re classes, as RE2 class bodies.
const (
	pySpace = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
	pyWord  = `\p{L}\p{N}_`
)

// Compile compiles a pattern written with Python's \s, \w and \d (inside or outside a
// bracket class); the rest of the syntax must already be RE2's.
func Compile(pattern string) *regexp.Regexp {
	bodies := map[byte]string{'s': pySpace, 'w': pyWord, 'd': `\p{Nd}`}
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			if body, ok := bodies[pattern[i+1]]; ok {
				if inClass {
					b.WriteString(body)
				} else {
					b.WriteString("[" + body + "]")
				}
			} else {
				b.WriteString(pattern[i : i+2])
			}
			i++
			continue
		case c == '[' && !inClass:
			inClass = true
		case c == ']' && inClass:
			inClass = false
		}
		b.WriteByte(c)
	}
	return regexp.MustCompile(b.String())
}

var (
	pkgRE   = Compile(`(?m)^\s*package\s+([\w.]+)\s*;`)
	classRE = Compile(`public\s+(?:final\s+|abstract\s+)*class\s+(\w+)(?:\s+extends\s+([\w.<>]+))?`)
	declRE  = Compile(`public\s+static\s+final\s+(Property|Action|Topic)\s+(\w+)\s*(=|;)\s*`)
	callRE  = Compile(`new(Property|Action|Topic)\s*\(`)
	spaceRE = Compile(`\s+`)
	castRE  = Compile(`(?s)^\((?:int|BValue|BFacets|String|long|double|boolean|BObject)\)\s*(.*)$`)
)

// ParseSource is parse_source: the package, the first public class (and what it extends) and
// every Property/Action/Topic declaration of text, in source order.
func ParseSource(text string) Type {
	var out Type
	if m := pkgRE.FindStringSubmatch(text); m != nil {
		out.Package = m[1]
	}
	if m := findClass(text); m != nil {
		out.Class, out.Extends = m[1], m[2]
	}
	for _, m := range declRE.FindAllStringSubmatchIndex(text, -1) {
		kind, name := text[m[2]:m[3]], text[m[4]:m[5]]
		start := m[1]
		if text[m[6]:m[7]] == ";" { // Procyon: declared bare, assigned in a static {} block
			at, ok := findAssign(text, name, start)
			if !ok {
				continue
			}
			start = at
		}
		expr := balancedCall(text, start)
		call := callRE.FindStringIndex(expr)
		if call == nil {
			continue
		}
		inner, ok := callArgs(expr, call[1])
		if !ok {
			continue
		}
		args := SplitArgs(inner)
		if len(args) == 0 {
			continue
		}
		flags, letters := DecodeFlags(Clean(args[0]))
		d := Decl{Name: name, Flags: flags, FlagLetters: strings.Join(letters, "")}
		for _, a := range args {
			d.Args = append(d.Args, Clean(a))
		}
		switch {
		case kind == "Property":
			d.Default, d.Facets = argOr(args, 1, ""), argOr(args, 2, "null")
		case len(args) == 2: // newAction(flags, facets) / newTopic(flags, facets): no parameter
			d.Facets = Clean(args[1])
		default:
			d.Default = argOr(args, 1, "")
			d.Facets = "null"
			if len(args) > 2 {
				d.Facets = Clean(args[len(args)-1])
			}
		}
		switch kind {
		case "Property":
			out.Properties = append(out.Properties, d)
		case "Action":
			out.Actions = append(out.Actions, d)
		default:
			out.Topics = append(out.Topics, d)
		}
	}
	return out
}

func argOr(args []string, i int, missing string) string {
	if i < len(args) {
		return Clean(args[i])
	}
	return missing
}

// findClass is classRE with Python's leading \b (a Unicode word boundary, which RE2's ASCII \b
// is not): a match preceded by a word character is skipped.
func findClass(text string) []string {
	for pos := 0; pos <= len(text); {
		loc := classRE.FindStringSubmatchIndex(text[pos:])
		if loc == nil {
			return nil
		}
		at := pos + loc[0]
		if r, ok := lastRune(text[:at]); !ok || !IsWord(r) {
			m := make([]string, 3)
			for g := 0; g < 3; g++ {
				if loc[2*g] >= 0 {
					m[g] = text[pos+loc[2*g] : pos+loc[2*g+1]]
				}
			}
			return m
		}
		pos = at + 1
	}
	return nil
}

func lastRune(s string) (rune, bool) {
	if s == "" {
		return 0, false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r, true
}

// findAssign is re.search(r"(?<![\w.])NAME\s*=(?!=)\s*", text, start): the offset after the match.
func findAssign(text, name string, start int) (int, bool) {
	for i := start; i <= len(text)-len(name); {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return 0, false
		}
		at := i + j
		i = at + 1
		if r, ok := lastRune(text[:at]); ok && (IsWord(r) || r == '.') {
			continue
		}
		k := skipSpace(text, at+len(name))
		if k >= len(text) || text[k] != '=' || (k+1 < len(text) && text[k+1] == '=') {
			continue
		}
		return skipSpace(text, k+1), true
	}
	return 0, false
}

func skipSpace(s string, i int) int {
	for i < len(s) {
		r, n := decodeRune(s[i:])
		if !IsSpace(r) {
			break
		}
		i += n
	}
	return i
}

// balancedCall is _balanced_call: from start, the text up to the ';' at paren depth 0
// (string and char literals skipped), or the rest of text.
func balancedCall(text string, start int) string {
	depth := 0
	var inStr byte
	for i := start; i < len(text); i++ {
		c := text[i]
		switch {
		case inStr != 0:
			if c == '\\' {
				i++
			} else if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ';' && depth == 0:
			return text[start:i]
		}
	}
	return text[start:]
}

// callArgs returns expr from start up to the ')' that closes the call opened just before start.
func callArgs(expr string, start int) (string, bool) {
	depth := 1
	var inStr byte
	for j := start; j < len(expr); j++ {
		c := expr[j]
		switch {
		case inStr != 0:
			if c == '\\' {
				j++
			} else if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return expr[start:j], true
			}
		}
	}
	return "", false
}

// SplitArgs is _split_args: s split at the top-level commas (parens, brackets, braces and
// string literals respected), each part stripped; "" gives no parts.
func SplitArgs(s string) []string {
	var args []string
	depth, cur := 0, 0
	var inStr byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr != 0:
			if c == '\\' && i+1 < len(s) {
				i++
			} else if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			args = append(args, Strip(s[cur:i]))
			cur = i + 1
		}
	}
	if tail := Strip(s[cur:]); tail != "" || len(args) > 0 {
		args = append(args, tail)
	}
	return args
}

// Clean is _clean: leading decompiler casts ((BValue)x, (int)0) dropped, whitespace runs
// collapsed to one space.
func Clean(expr string) string {
	e := Strip(expr)
	for {
		m := castRE.FindStringSubmatch(e)
		if m == nil {
			break
		}
		e = Strip(m[1])
	}
	return spaceRE.ReplaceAllLiteralString(e, " ")
}
