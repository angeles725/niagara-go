package pyre

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	var lits strings.Builder
	var caseless, comps []string // ASCII category classes; complements of \D \S \W
	for _, it := range n.set {
		switch {
		case it.isCat:
			if body, neg := complemented(it.cat, ascii); neg {
				comps = append(comps, body)
			} else if ascii {
				caseless = append(caseless, categoryInClass(it.cat, ascii))
			} else {
				lits.WriteString(categoryInClass(it.cat, ascii))
			}
		case it.isRange:
			lits.WriteString(escapeRune(it.lo) + "-" + escapeRune(it.hi))
		default:
			lits.WriteString(escapeRune(it.lo))
		}
	}
	b := &e.b
	if len(caseless) == 0 && len(comps) == 0 {
		if n.negate {
			b.WriteString("[^" + lits.String() + "]")
		} else {
			b.WriteString("[" + lits.String() + "]")
		}
		return
	}
	cls := func(body string, neg bool) string {
		c := "[" + body + "]"
		if neg {
			c = "[^" + body + "]"
		}
		return asciiOnly(c, ascii)
	}
	b.WriteString("(?:")
	if !n.negate { // any member: the literals, a category, a complement
		var alts []string
		if lits.Len() > 0 {
			alts = append(alts, "["+lits.String()+"]")
		}
		for _, c := range caseless {
			alts = append(alts, cls(c, false))
		}
		for _, c := range comps {
			alts = append(alts, cls(c, true))
		}
		b.WriteString(strings.Join(alts, "|") + ")")
		return
	}
	// no member: not a literal, not in a category, in every complemented set
	if lits.Len() > 0 {
		b.WriteString("(?![" + lits.String() + "])")
	}
	for _, c := range caseless {
		b.WriteString("(?!" + cls(c, false) + ")")
	}
	if len(comps) == 0 {
		b.WriteString(`[\s\S])`)
		return
	}
	for _, c := range comps[:len(comps)-1] {
		b.WriteString("(?=" + cls(c, false) + ")")
	}
	b.WriteString(cls(comps[len(comps)-1], false) + ")")
}

// asciiOnly keeps an re.ASCII class out of IGNORECASE: .NET would lower-case the input
// first (U+212A KELVIN SIGN to "k"), Python folds ASCII letters only.
func asciiOnly(class string, ascii bool) string {
	if ascii {
		return "(?-i:" + class + ")"
	}
	return class
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

// Python str patterns: \s is sre's Unicode whitespace (str.isspace: .NET \s lacks
// \x1c-\x1f), \d is Unicode Nd and \w is str.isalnum() or "_", all in Python's Unicode
// version; they are written as explicit classes from classes_gen.go (generated from
// Python's re), since regexp2's classes follow .NET definitions and Go's Unicode tables.
// With re.ASCII the three are the ASCII sets.
const (
	asciiDigit = `0-9`
	asciiSpace = ` \t\n\r\f\v`
	asciiWord  = `a-zA-Z0-9_`
)

// classBody writes inclusive (lo, hi) pairs as a .NET class body.
func classBody(pairs []rune) string {
	var b strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		b.WriteString(escapeRune(pairs[i]))
		if pairs[i+1] != pairs[i] {
			b.WriteString("-" + escapeRune(pairs[i+1]))
		}
	}
	return b.String()
}

// nonASCII keeps the pairs at or above U+0080.
func nonASCII(pairs []rune) []rune {
	var out []rune
	for i := 0; i < len(pairs); i += 2 {
		if pairs[i+1] >= 0x80 {
			out = append(out, max(pairs[i], 0x80), pairs[i+1])
		}
	}
	return out
}

var (
	pySpace = sync.OnceValue(func() string { return classBody(pySpaceRanges) })
	pyDigit = sync.OnceValue(func() string { return classBody(pyDigitRanges) })
	pyWord  = sync.OnceValue(func() string { return classBody(pyWordRanges) })
	// regexp2 scans a class's ranges one by one, so the \d and \w atoms test ASCII
	// first and reach the long non-ASCII range list only for a non-ASCII character.
	pyDigitHigh = sync.OnceValue(func() string { return classBody(nonASCII(pyDigitRanges)) })
	pyWordHigh  = sync.OnceValue(func() string { return classBody(nonASCII(pyWordRanges)) })
)

const isHigh = `(?![\u0000-\u007F])`

// fastAtom is the \d / \D / \w / \W atom as an ASCII test, then the non-ASCII ranges.
func fastAtom(c category) (string, bool) {
	var low, high string
	switch c {
	case catDigit, catNotDigit:
		low, high = asciiDigit, pyDigitHigh()
	case catWord, catNotWord:
		low, high = asciiWord, pyWordHigh()
	default:
		return "", false
	}
	if isNegated(c) {
		return "(?:(?![" + low + `])[\u0000-\u007F]|` + isHigh + "[^" + high + "])", true
	}
	return "(?:[" + low + "]|" + isHigh + "[" + high + "])", true
}

// positiveBody is the class body of \d, \s or \w.
func positiveBody(c category, ascii bool) string {
	switch {
	case ascii && (c == catDigit || c == catNotDigit):
		return asciiDigit
	case ascii && (c == catSpace || c == catNotSpace):
		return asciiSpace
	case ascii:
		return asciiWord
	case c == catDigit || c == catNotDigit:
		return pyDigit()
	case c == catSpace || c == catNotSpace:
		return pySpace()
	}
	return pyWord()
}

func isNegated(c category) bool { return c == catNotDigit || c == catNotSpace || c == catNotWord }

func categoryAtom(c category, ascii bool) string {
	if atom, ok := fastAtom(c); ok && !ascii {
		return atom
	}
	if isNegated(c) {
		return asciiOnly("[^"+positiveBody(c, ascii)+"]", ascii)
	}
	return asciiOnly("["+positiveBody(c, ascii)+"]", ascii)
}

// categoryInClass is the class body of a positive category (\d, \s, \w).
func categoryInClass(c category, ascii bool) string { return positiveBody(c, ascii) }

// complemented reports the negated categories (\D, \S, \W), which a .NET class cannot
// hold as a union member, and the body of the set they complement.
func complemented(c category, ascii bool) (string, bool) {
	if isNegated(c) {
		return positiveBody(c, ascii), true
	}
	return "", false
}

// anchor writes ^ $ \A \z and the word boundaries; \b and \B are lookarounds on
// Python's \w (ASCII's under re.ASCII), not the .NET word definition.
func anchor(a atKind, ascii bool) string {
	if a == atBoundary || a == atNonBoundary {
		w := categoryAtom(catWord, ascii)
		if a == atBoundary {
			return "(?:(?<=" + w + ")(?!" + w + ")|(?<!" + w + ")(?=" + w + "))"
		}
		return "(?:(?<=" + w + ")(?=" + w + ")|(?<!" + w + ")(?!" + w + "))"
	}
	return map[atKind]string{atBeginning: "^", atEnd: "$", atBeginningString: `\A`, atEndString: `\z`}[a]
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
