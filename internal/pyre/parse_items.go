package pyre

import (
	"strconv"
	"strings"
	"unicode"
)

// _parse_sub, _parse, _parse_flags and parse() of CPython's re/_parser.py.

func parse(pattern string, flags Flags) (seq []*node, st *state, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			seq, st, err = nil, nil, e
		}
	}()
	t := newTokenizer(pattern)
	st = &state{flags: flags, groupdict: map[string]int{}, groupwidths: [][2]int{{}}, closed: []bool{false},
		groupnames: []string{""}, lookbehindgroups: -1, grouprefpos: map[int]int{}}
	seq = parseSub(t, st, flags&VERBOSE != 0, 0)
	fixFlags(st)
	if t.next != nil {
		panic(t.error("unbalanced parenthesis", 0))
	}
	for _, g := range st.grouprefOrder {
		if g >= st.groups() {
			panic(newError("invalid group reference "+strconv.Itoa(g), pattern, st.grouprefpos[g]))
		}
	}
	return seq, st, nil
}

// fixFlags is _parser.fix_flags for a str pattern (its ValueError carries no position).
func fixFlags(st *state) {
	switch {
	case st.flags&LOCALE != 0:
		panic(newError("cannot use LOCALE flag with a str pattern", "", -1))
	case st.flags&ASCII != 0 && st.flags&UNICODE != 0:
		panic(newError("ASCII and UNICODE flags are incompatible", "", -1))
	}
}

func parseSub(t *tokenizer, st *state, verbose bool, nested int) []*node {
	var items [][]*node
	for {
		items = append(items, parseSeq(t, st, verbose, nested+1, nested == 0 && len(items) == 0))
		if !t.match("|") {
			break
		}
		if nested == 0 {
			verbose = st.flags&VERBOSE != 0
		}
	}
	if len(items) == 1 {
		return items[0]
	}
	return []*node{{kind: kBranch, branches: items}}
}

func isQuantifier(n *node) bool { return n.kind == kRepeat }

func parseSeq(t *tokenizer, st *state, verbose bool, nested int, first bool) []*node {
	var seq []*node
	for {
		if t.next == nil || t.is("|") || t.is(")") {
			break
		}
		this := t.get()
		if verbose {
			if len(this) == 1 && containsRune(spaces, this[0]) {
				continue
			}
			if string(this) == "#" {
				for {
					c := t.get()
					if c == nil || string(c) == "\n" {
						break
					}
				}
				continue
			}
		}
		switch {
		case this[0] == '\\':
			seq = append(seq, t.escape(this, st))
		case !containsRune(special, this[0]):
			seq = append(seq, &node{kind: kLiteral, r: this[0]})
		case this[0] == '[':
			seq = append(seq, parseSet(t))
		case containsRune(repeats, this[0]):
			seq = parseRepeat(t, seq, this[0])
		case this[0] == '.':
			seq = append(seq, &node{kind: kAny})
		case this[0] == '(':
			var n *node
			var isGlobal bool
			if n, isGlobal = parseGroup(t, st, verbose, nested, first, len(seq) > 0); isGlobal {
				verbose = st.flags&VERBOSE != 0
				continue
			}
			if n != nil {
				seq = append(seq, n)
			}
		case this[0] == '^':
			seq = append(seq, &node{kind: kAt, at: atBeginning})
		case this[0] == '$':
			seq = append(seq, &node{kind: kAt, at: atEnd})
		}
	}
	return seq
}

func parseSet(t *tokenizer) *node {
	here := t.tell() - 1
	var set []setItem
	negate := t.match("^")
	unterminated := func() *Error { return t.error("unterminated character set", t.tell()-here) }
	for {
		this := t.get()
		if this == nil {
			panic(unterminated())
		}
		if string(this) == "]" && len(set) > 0 {
			break
		}
		var code1 setItem
		if this[0] == '\\' && len(this) == 2 {
			code1 = t.classEscape(this)
		} else {
			code1 = setItem{lo: this[0]}
		}
		if t.match("-") {
			that := t.get()
			if that == nil {
				panic(unterminated())
			}
			if string(that) == "]" {
				set = append(set, code1, setItem{lo: '-'})
				break
			}
			var code2 setItem
			if that[0] == '\\' && len(that) == 2 {
				code2 = t.classEscape(that)
			} else {
				code2 = setItem{lo: that[0]}
			}
			msg := "bad character range " + string(this) + "-" + string(that)
			if code1.isCat || code2.isCat || code2.lo < code1.lo {
				panic(t.error(msg, len(this)+1+len(that)))
			}
			set = append(set, setItem{isRange: true, lo: code1.lo, hi: code2.lo})
		} else {
			set = append(set, code1)
		}
	}
	if len(set) == 1 && !set[0].isCat && !set[0].isRange {
		if negate {
			return &node{kind: kNotLiteral, r: set[0].lo}
		}
		return &node{kind: kLiteral, r: set[0].lo}
	}
	return &node{kind: kIn, set: set, negate: negate}
}

// parseRepeat applies a quantifier to the last item; a "{" that does not start a
// valid {m,n} is appended to the returned sequence as a literal.
func parseRepeat(t *tokenizer, seq []*node, this rune) []*node {
	here := t.tell()
	lo, hi := 0, maxRepeat
	switch this {
	case '?':
		hi = 1
	case '+':
		lo = 1
	case '{':
		if t.is("}") {
			return append(seq, &node{kind: kLiteral, r: '{'})
		}
		los := t.getwhile(1<<30, digits)
		his := los
		if t.match(",") {
			his = t.getwhile(1<<30, digits)
		}
		if !t.match("}") {
			seq = append(seq, &node{kind: kLiteral, r: '{'})
			t.seek(here)
			return seq
		}
		if los != "" {
			lo = atoiRepeat(t, los)
		}
		if his != "" {
			hi = atoiRepeat(t, his)
			if hi < lo {
				panic(t.error("min repeat greater than max repeat", t.tell()-here))
			}
		}
	}
	if len(seq) == 0 || seq[len(seq)-1].kind == kAt {
		panic(t.error("nothing to repeat", t.tell()-here+1))
	}
	last := seq[len(seq)-1]
	if isQuantifier(last) {
		panic(t.error("multiple repeat", t.tell()-here+1))
	}
	item := []*node{last}
	if last.kind == kGroup && last.group == 0 && last.add == 0 && last.del == 0 {
		item = last.sub
	}
	mode := greedy
	if t.match("?") {
		mode = lazy
	} else if t.match("+") {
		mode = possessive
	}
	seq[len(seq)-1] = &node{kind: kRepeat, min: lo, max: hi, mode: mode, sub: item}
	return seq
}

func atoiRepeat(t *tokenizer, s string) int {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v >= maxRepeat {
		panic(newError("the repetition number is too large", "", -1))
	}
	return int(v)
}

func parseConditional(t *tokenizer, st *state, verbose bool, nested, start int) *node {
	condname := t.getuntil(')', "group name")
	nl := len([]rune(condname))
	var cond int
	if isASCIIDigits(condname) {
		v, err := strconv.ParseInt(condname, 10, 64)
		if err == nil && v == 0 {
			panic(t.error("bad group number", nl+1))
		}
		if err != nil || v >= maxGroups {
			digits := strings.TrimLeft(condname, "0")
			panic(t.error("invalid group reference "+digits, nl+1))
		}
		cond = int(v)
		if _, ok := st.grouprefpos[cond]; !ok {
			st.grouprefpos[cond] = t.tell() - nl - 1
			st.grouprefOrder = append(st.grouprefOrder, cond)
		}
	} else {
		t.checkgroupname(condname, 1)
		g, ok := st.groupdict[condname]
		if !ok {
			panic(t.error("unknown group name "+pyRepr(condname), nl+1))
		}
		cond = g
	}
	st.checklookbehindgroup(cond, t)
	n := &node{kind: kGroupRefExists, ref: cond, yes: parseSeq(t, st, verbose, nested+1, false)}
	if t.match("|") {
		n.no, n.hasNo = parseSeq(t, st, verbose, nested+1, false), true
		if t.is("|") {
			panic(t.error("conditional backref with more than two branches", 0))
		}
	}
	if !t.match(")") {
		panic(t.error("missing ), unterminated subpattern", t.tell()-start))
	}
	return n
}

func isASCIIDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func parseFlags(t *tokenizer, st *state, char rune) (add, del Flags, global bool) {
	c := char
	if c != '-' {
		for {
			if c == 'L' {
				panic(t.error("bad inline flags: cannot use 'L' flag with a str pattern", 0))
			}
			f := flagOf(c)
			add |= f
			if f&typeFlags != 0 && add&typeFlags != f {
				panic(t.error("bad inline flags: flags 'a', 'u' and 'L' are incompatible", 0))
			}
			next := t.get()
			if next == nil {
				panic(t.error("missing -, : or )", 0))
			}
			c = next[0]
			if len(next) == 1 && containsRune(")-:", c) {
				break
			}
			if len(next) != 1 || flagOf(c) == 0 {
				panic(t.error(unknownFlag(next, "missing -, : or )"), len(next)))
			}
		}
	}
	if c == ')' {
		st.flags |= add
		return 0, 0, true
	}
	if c == '-' {
		next := t.get()
		if next == nil {
			panic(t.error("missing flag", 0))
		}
		if len(next) != 1 || flagOf(next[0]) == 0 {
			panic(t.error(unknownFlag(next, "missing flag"), len(next)))
		}
		c = next[0]
		for {
			f := flagOf(c)
			if f&typeFlags != 0 {
				panic(t.error("bad inline flags: cannot turn off flags 'a', 'u' and 'L'", 0))
			}
			del |= f
			next := t.get()
			if next == nil {
				panic(t.error("missing :", 0))
			}
			c = next[0]
			if string(next) == ":" {
				break
			}
			if len(next) != 1 || flagOf(c) == 0 {
				panic(t.error(unknownFlag(next, "missing :"), len(next)))
			}
		}
	}
	if add&del != 0 {
		panic(t.error("bad inline flags: flag turned on and off", 1))
	}
	return add, del, false
}

func unknownFlag(tok []rune, otherwise string) string {
	if len(tok) == 1 && unicode.IsLetter(tok[0]) {
		return "unknown flag"
	}
	return otherwise
}
