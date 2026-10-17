package pyre

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// pyRepr is repr() of a str (re.error quotes group and character names with it).
var pyRepr = pycompat.StrRepr

// This file ports CPython 3.15 Lib/re/_parser.py: the same grammar, checked in the
// same order, so an invalid pattern fails with the same message and position. The
// result is a small tree that emit.go turns into a regexp2 (.NET) pattern. Parse
// errors are raised as panics carrying *Error and recovered in parse().

// Python's limits: MAXREPEAT is the repeat bound of _sre, MAXGROUPS the group count.
const (
	maxRepeat = 1<<32 - 1
	maxGroups = 1<<31 - 1
	maxWidth  = 1 << 62 // stands for MAXWIDTH (1 << 64); only lo != hi is ever tested
)

type kind int

const (
	kLiteral kind = iota
	kNotLiteral
	kAny
	kIn
	kAt
	kBranch
	kRepeat
	kGroup // SUBPATTERN: capturing (group > 0) or flag-scoped / plain non-capturing
	kAtomic
	kAssert
	kAssertNot
	kFailure
	kGroupRef
	kGroupRefExists
)

type atKind int

const (
	atBeginning       atKind = iota // ^
	atEnd                           // $
	atBeginningString               // \A
	atEndString                     // \z, \Z
	atBoundary                      // \b
	atNonBoundary                   // \B
)

type category int

const (
	catDigit category = iota
	catNotDigit
	catSpace
	catNotSpace
	catWord
	catNotWord
)

type repeatMode int

const (
	greedy repeatMode = iota
	lazy
	possessive
)

// setItem is one member of a character set: a literal, a range or a category.
type setItem struct {
	isRange, isCat bool
	lo, hi         rune
	cat            category
}

// node is one parsed item; which fields are used depends on kind.
type node struct {
	kind     kind
	r        rune       // kLiteral, kNotLiteral
	set      []setItem  // kIn
	negate   bool       // kIn
	at       atKind     // kAt
	branches [][]*node  // kBranch
	min, max int        // kRepeat
	mode     repeatMode // kRepeat
	sub      []*node    // kRepeat item, kGroup / kAtomic / kAssert* contents
	group    int        // kGroup: 0 = non-capturing
	add, del Flags      // kGroup: scoped flags
	dir      int        // kAssert*: 1 ahead, -1 behind
	ref      int        // kGroupRef, kGroupRefExists
	yes, no  []*node    // kGroupRefExists
	hasNo    bool
}

type state struct {
	flags            Flags
	groupdict        map[string]int
	groupnames       []string // index = group number ("" = unnamed)
	groupwidths      [][2]int // index = group number; closed groups only
	closed           []bool
	lookbehindgroups int // -1 = not inside a lookbehind
	grouprefpos      map[int]int
	grouprefOrder    []int
}

func (st *state) groups() int { return len(st.groupwidths) }

type tokenizer struct {
	pattern string
	s       []rune
	index   int
	next    []rune // nil = end of pattern
}

func newTokenizer(pattern string) *tokenizer {
	t := &tokenizer{pattern: pattern, s: []rune(pattern)}
	t.advance()
	return t
}

func (t *tokenizer) advance() {
	i := t.index
	if i >= len(t.s) {
		t.next = nil
		return
	}
	c := []rune{t.s[i]}
	if c[0] == '\\' {
		i++
		if i >= len(t.s) {
			panic(newError("bad escape (end of pattern)", t.pattern, len(t.s)-1))
		}
		c = append(c, t.s[i])
	}
	t.index = i + 1
	t.next = c
}

func (t *tokenizer) is(s string) bool { return t.next != nil && string(t.next) == s }

func (t *tokenizer) match(s string) bool {
	if t.is(s) {
		t.advance()
		return true
	}
	return false
}

func (t *tokenizer) get() []rune {
	this := t.next
	t.advance()
	return this
}

func (t *tokenizer) getwhile(n int, set string) string {
	out := ""
	for range n {
		if t.next == nil || len(t.next) != 1 || !containsRune(set, t.next[0]) {
			break
		}
		out += string(t.next)
		t.advance()
	}
	return out
}

func (t *tokenizer) getuntil(terminator rune, name string) string {
	var result []rune
	for {
		c := t.next
		t.advance()
		if c == nil {
			if len(result) == 0 {
				panic(t.error("missing "+name, 0))
			}
			panic(t.error(fmt.Sprintf("missing %c, unterminated name", terminator), len(result)))
		}
		if len(c) == 1 && c[0] == terminator {
			if len(result) == 0 {
				panic(t.error("missing "+name, 1))
			}
			break
		}
		result = append(result, c...)
	}
	return string(result)
}

func (t *tokenizer) tell() int { return t.index - len(t.next) }

func (t *tokenizer) seek(i int) {
	t.index = i
	t.advance()
}

func (t *tokenizer) error(msg string, offset int) *Error {
	return newError(msg, t.pattern, t.tell()-offset)
}

func (t *tokenizer) checkgroupname(name string, offset int) {
	if !isIdentifier(name) {
		panic(t.error("bad character in group name "+pyRepr(name), len([]rune(name))+offset))
	}
}

// isIdentifier approximates str.isidentifier: a letter or '_' then letters, digits, '_'.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range []rune(s) {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && (unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) ||
			unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Pc, r))) {
			continue
		}
		return false
	}
	return true
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

const (
	digits    = "0123456789"
	octdigits = "01234567"
	hexdigits = "0123456789abcdefABCDEF"
	special   = ".\\[{()*+?^$|"
	repeats   = "*+?{"
	spaces    = " \t\n\r\v\f"
)

func isASCIILetter(r rune) bool { return r < 128 && unicode.IsLetter(r) }

var escapes = map[rune]rune{'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', '\\': '\\'}

var categories = map[rune]category{'d': catDigit, 'D': catNotDigit, 's': catSpace, 'S': catNotSpace,
	'w': catWord, 'W': catNotWord}

var anchors = map[rune]atKind{'A': atBeginningString, 'b': atBoundary, 'B': atNonBoundary, 'z': atEndString,
	'Z': atEndString}

// hexEscape parses the digits of \x, \u or \U (escape is the 2-rune token).
func (t *tokenizer) hexEscape(escape []rune) (rune, bool) {
	n := map[rune]int{'x': 2, 'u': 4, 'U': 8}[escape[1]]
	if n == 0 {
		return 0, false
	}
	text := string(escape) + t.getwhile(n, hexdigits)
	if len([]rune(text)) != n+2 {
		panic(t.error("incomplete escape "+text, len([]rune(text))))
	}
	v, _ := strconv.ParseUint(text[2:], 16, 32)
	if v > unicode.MaxRune {
		panic(t.error("bad escape "+text, len([]rune(text))))
	}
	return rune(v), true
}

func (t *tokenizer) namedEscape() rune {
	if !t.match("{") {
		panic(t.error("missing {", 0))
	}
	name := t.getuntil('}', "character name")
	panic(t.error("undefined character name "+pyRepr(name), len([]rune(name))+len(`\N{}`)))
}

func (t *tokenizer) classEscape(escape []rune) setItem {
	c := escape[1]
	if r, ok := escapes[c]; ok {
		return setItem{lo: r}
	}
	if cat, ok := categories[c]; ok {
		return setItem{isCat: true, cat: cat}
	}
	if r, ok := t.hexEscape(escape); ok {
		return setItem{lo: r}
	}
	switch {
	case c == 'N':
		t.namedEscape()
	case containsRune(octdigits, c):
		text := string(escape) + t.getwhile(2, octdigits)
		v, _ := strconv.ParseInt(text[1:], 8, 32)
		if v > 0o377 {
			panic(t.error(fmt.Sprintf("octal escape value %s outside of range 0-0o377", text), len([]rune(text))))
		}
		return setItem{lo: rune(v)}
	case containsRune(digits, c):
	case !isASCIILetter(c):
		return setItem{lo: c}
	}
	panic(t.error("bad escape "+string(escape), len(escape)))
}

func (t *tokenizer) escape(escape []rune, st *state) *node {
	c := escape[1]
	if at, ok := anchors[c]; ok {
		return &node{kind: kAt, at: at}
	}
	if cat, ok := categories[c]; ok {
		return &node{kind: kIn, set: []setItem{{isCat: true, cat: cat}}}
	}
	if r, ok := escapes[c]; ok {
		return &node{kind: kLiteral, r: r}
	}
	if r, ok := t.hexEscape(escape); ok {
		return &node{kind: kLiteral, r: r}
	}
	switch {
	case c == 'N':
		t.namedEscape()
	case c == '0':
		text := string(escape) + t.getwhile(2, octdigits)
		v, _ := strconv.ParseInt(text[1:], 8, 32)
		return &node{kind: kLiteral, r: rune(v)}
	case containsRune(digits, c):
		text := string(escape)
		if t.next != nil && len(t.next) == 1 && containsRune(digits, t.next[0]) {
			text += string(t.get())
			if containsRune(octdigits, rune(text[1])) && containsRune(octdigits, rune(text[2])) &&
				t.next != nil && len(t.next) == 1 && containsRune(octdigits, t.next[0]) {
				text += string(t.get())
				v, _ := strconv.ParseInt(text[1:], 8, 32)
				if v > 0o377 {
					panic(t.error(fmt.Sprintf("octal escape value %s outside of range 0-0o377", text), len(text)))
				}
				return &node{kind: kLiteral, r: rune(v)}
			}
		}
		group, _ := strconv.Atoi(text[1:])
		if group < st.groups() {
			if !st.checkgroup(group) {
				panic(t.error("cannot refer to an open group", len(text)))
			}
			st.checklookbehindgroup(group, t)
			return &node{kind: kGroupRef, ref: group}
		}
		panic(t.error(fmt.Sprintf("invalid group reference %d", group), len(text)-1))
	case !isASCIILetter(c):
		return &node{kind: kLiteral, r: c}
	}
	panic(t.error("bad escape "+string(escape), len(escape)))
}

func (st *state) opengroup(name string) (int, string) {
	gid := st.groups()
	st.groupwidths = append(st.groupwidths, [2]int{})
	st.closed = append(st.closed, false)
	st.groupnames = append(st.groupnames, name)
	if st.groups() > maxGroups {
		return 0, "too many groups"
	}
	if name != "" {
		if ogid, ok := st.groupdict[name]; ok {
			return 0, fmt.Sprintf("redefinition of group name %s as group %d; was group %d", pyRepr(name), gid, ogid)
		}
		st.groupdict[name] = gid
	}
	return gid, ""
}

func (st *state) checkgroup(gid int) bool { return gid < st.groups() && st.closed[gid] }

func (st *state) checklookbehindgroup(gid int, t *tokenizer) {
	if st.lookbehindgroups >= 0 {
		if !st.checkgroup(gid) {
			panic(t.error("cannot refer to an open group", 0))
		}
		if gid >= st.lookbehindgroups {
			panic(t.error("cannot refer to group defined in the same lookbehind subpattern", 0))
		}
	}
}
