package pyre

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// emitter writes the regexp2 (.NET syntax) form of a parsed Python pattern. Every
// capturing group becomes an unnamed group, so .NET numbers them like Python does.
type emitter struct{ b strings.Builder }

func (e *emitter) seq(seq []*node, fl Flags) {
	for _, n := range seq {
		e.node(n, fl)
	}
}

func (e *emitter) node(n *node, fl Flags) {
	b := &e.b
	switch n.kind {
	case kLiteral:
		b.WriteString(escapeRune(n.r))
	case kNotLiteral:
		b.WriteString("[^" + escapeRune(n.r) + "]")
	case kAny:
		b.WriteString(".")
	case kIn:
		if len(n.set) == 1 && n.set[0].isCat && !n.negate {
			b.WriteString(categoryAtom(n.set[0].cat, fl&ASCII != 0))
			return
		}
		e.set(n, fl&ASCII != 0)
	case kAt:
		b.WriteString(anchor(n.at, fl&ASCII != 0))
	case kBranch:
		b.WriteString("(?:")
		for i, br := range n.branches {
			if i > 0 {
				b.WriteString("|")
			}
			e.seq(br, fl)
		}
		b.WriteString(")")
	case kRepeat:
		e.repeat(n, fl)
	case kGroup:
		inner := (fl | n.add) &^ n.del
		switch {
		case n.group > 0:
			b.WriteString("(")
		default:
			b.WriteString("(?" + inlineFlags(n.add, n.del) + ":")
		}
		e.seq(n.sub, inner)
		b.WriteString(")")
	case kAtomic:
		e.wrap("(?>", n.sub, fl)
	case kAssert:
		e.wrap(map[int]string{1: "(?=", -1: "(?<="}[n.dir], n.sub, fl)
	case kAssertNot:
		e.wrap(map[int]string{1: "(?!", -1: "(?<!"}[n.dir], n.sub, fl)
	case kFailure:
		b.WriteString("(?!)")
	case kGroupRef:
		b.WriteString(`\k<` + strconv.Itoa(n.ref) + ">")
	case kGroupRefExists:
		b.WriteString("(?(" + strconv.Itoa(n.ref) + ")")
		e.seq(n.yes, fl)
		b.WriteString("|")
		e.seq(n.no, fl)
		b.WriteString(")")
	}
}

// set writes a character set. A .NET class cannot hold the complement of a Python
// category whose set differs from the .NET one ([\W], [\S], [\D] under re.ASCII), so
// those become alternatives: [x\W] is (?:[x]|[^word]) and [^x\W] is (?![x])[word].
func (e *emitter) set(n *node, ascii bool) {
	var pos strings.Builder
	var comps []string
	for _, it := range n.set {
		switch {
		case it.isCat:
			if body, neg := complemented(it.cat, ascii); neg {
				comps = append(comps, body)
			} else {
				pos.WriteString(categoryInClass(it.cat, ascii))
			}
		case it.isRange:
			pos.WriteString(escapeRune(it.lo) + "-" + escapeRune(it.hi))
		default:
			pos.WriteString(escapeRune(it.lo))
		}
	}
	b := &e.b
	switch {
	case len(comps) == 0 && n.negate:
		b.WriteString("[^" + pos.String() + "]")
	case len(comps) == 0:
		b.WriteString("[" + pos.String() + "]")
	case !n.negate:
		b.WriteString("(?:")
		if pos.Len() > 0 {
			b.WriteString("[" + pos.String() + "]|")
		}
		b.WriteString("[^" + strings.Join(comps, "]|[^") + "])")
	default:
		b.WriteString("(?:")
		if pos.Len() > 0 {
			b.WriteString("(?![" + pos.String() + "])")
		}
		for _, c := range comps[:len(comps)-1] {
			b.WriteString("(?=[" + c + "])")
		}
		b.WriteString("[" + comps[len(comps)-1] + "])")
	}
}

func (e *emitter) wrap(open string, sub []*node, fl Flags) {
	e.b.WriteString(open)
	e.seq(sub, fl)
	e.b.WriteString(")")
}

func (e *emitter) repeat(n *node, fl Flags) {
	var q string
	switch {
	case n.min == 0 && n.max == maxRepeat:
		q = "*"
	case n.min == 1 && n.max == maxRepeat:
		q = "+"
	case n.min == 0 && n.max == 1:
		q = "?"
	case n.max == maxRepeat:
		q = fmt.Sprintf("{%d,}", n.min)
	case n.min == n.max:
		q = fmt.Sprintf("{%d}", n.min)
	default:
		q = fmt.Sprintf("{%d,%d}", n.min, n.max)
	}
	if n.mode == possessive {
		e.b.WriteString("(?>")
	}
	e.wrap("(?:", n.sub, fl)
	e.b.WriteString(q)
	switch n.mode {
	case lazy:
		e.b.WriteString("?")
	case possessive:
		e.b.WriteString(")")
	}
}

// inlineFlags is the .NET spelling of a scoped flag group; VERBOSE and ASCII are
// applied by the parser and the emitter, so only i, m and s reach the engine.
func inlineFlags(add, del Flags) string {
	letters := func(f Flags) string {
		s := ""
		for _, x := range []struct {
			f Flags
			c string
		}{{IGNORECASE, "i"}, {MULTILINE, "m"}, {DOTALL, "s"}} {
			if f&x.f != 0 {
				s += x.c
			}
		}
		return s
	}
	on, off := letters(add), letters(del)
	if off != "" {
		return on + "-" + off
	}
	return on
}

// Python str patterns: \d is Unicode Nd (as .NET), \s is str.isspace (.NET \s lacks
// \x1c-\x1f), \w is str.isalnum() or "_" (letters and every number category). With
// re.ASCII the three are the ASCII sets.
const (
	pySpace     = `\s\u001c-\u001f`
	pyWord      = `\p{L}\p{N}_`
	asciiDigit  = `0-9`
	asciiSpace  = ` \t\n\r\f\v`
	asciiWord   = `a-zA-Z0-9_`
	asciiWordCl = `[a-zA-Z0-9_]`
)

func categoryAtom(c category, ascii bool) string {
	if ascii {
		return map[category]string{catDigit: "[" + asciiDigit + "]", catNotDigit: "[^" + asciiDigit + "]",
			catSpace: "[" + asciiSpace + "]", catNotSpace: "[^" + asciiSpace + "]",
			catWord: asciiWordCl, catNotWord: "[^" + asciiWord + "]"}[c]
	}
	return map[category]string{catDigit: `\d`, catNotDigit: `\D`, catSpace: "[" + pySpace + "]",
		catNotSpace: "[^" + pySpace + "]", catWord: "[" + pyWord + "]", catNotWord: "[^" + pyWord + "]"}[c]
}

// categoryInClass is the class body of a category that a .NET class can hold.
func categoryInClass(c category, ascii bool) string {
	if ascii {
		return map[category]string{catDigit: asciiDigit, catSpace: asciiSpace, catWord: asciiWord}[c]
	}
	return map[category]string{catDigit: `\d`, catNotDigit: `\D`, catSpace: pySpace, catWord: pyWord}[c]
}

// complemented reports the categories a class cannot hold (their complement differs
// from .NET's) and the body of the set they complement.
func complemented(c category, ascii bool) (string, bool) {
	switch {
	case c == catNotWord && ascii:
		return asciiWord, true
	case c == catNotWord:
		return pyWord, true
	case c == catNotSpace && ascii:
		return asciiSpace, true
	case c == catNotSpace:
		return pySpace, true
	case c == catNotDigit && ascii:
		return asciiDigit, true
	}
	return "", false
}

func anchor(a atKind, ascii bool) string {
	if ascii && (a == atBoundary || a == atNonBoundary) {
		w := asciiWordCl
		if a == atBoundary {
			return "(?:(?<=" + w + ")(?!" + w + ")|(?<!" + w + ")(?=" + w + "))"
		}
		return "(?:(?<=" + w + ")(?=" + w + ")|(?<!" + w + ")(?!" + w + "))"
	}
	return map[atKind]string{atBeginning: "^", atEnd: "$", atBeginningString: `\A`, atEndString: `\z`,
		atBoundary: `\b`, atNonBoundary: `\B`}[a]
}

// escapeRune writes one literal character so .NET reads it as itself, in or out of a set.
func escapeRune(r rune) string {
	switch {
	case r < 0x20 || r == 0x7f:
		return fmt.Sprintf(`\u%04X`, r)
	case r < 0x80 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != ' ':
		return `\` + string(r)
	}
	return string(r)
}
