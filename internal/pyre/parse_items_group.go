package pyre

// The "(" item of CPython's re/_parser.py _parse: after "(?" an extension either is the
// whole item (a group reference, a comment, a lookaround, a conditional, global flags) or
// sets up the group whose contents are then parsed like a capturing group's.

// groupSpec is what an extension leaves for the group contents: _parse's group (None for a
// non-capturing group), name, add_flags/del_flags, and atomic for "(?>".
type groupSpec struct {
	capture, atomic bool
	name            string
	add, del        Flags
}

// parseGroup handles "(" ... ")"; global is true for a leading (?flags) group.
func parseGroup(t *tokenizer, st *state, verbose bool, nested int, first, nonEmpty bool) (n *node, global bool) {
	start := t.tell() - 1
	g := groupSpec{capture: true}
	if t.match("?") {
		var done bool
		if n, global, done = parseExtension(t, st, verbose, nested, start, first, nonEmpty, &g); done {
			return n, global
		}
	}
	return parseGroupBody(t, st, verbose, nested, start, g), false
}

// unterminatedGroup is the error of a group whose ")" is missing, spanning from its "(".
func unterminatedGroup(t *tokenizer, start int) *Error {
	return t.error("missing ), unterminated subpattern", t.tell()-start)
}

// parseExtension reads the extension after "(?". done is true when it is the whole item
// (n, or nil for a comment or global flags, global true for the latter); otherwise it has
// set g up for the group contents.
func parseExtension(t *tokenizer, st *state, verbose bool, nested, start int, first, nonEmpty bool,
	g *groupSpec) (n *node, global, done bool) {
	char := t.get()
	if char == nil {
		panic(t.error("unexpected end of pattern", 0))
	}
	c := string(char)
	switch {
	case c == "P":
		var ref *node
		if g.name, ref = parsePExtension(t, st); ref != nil {
			return ref, false, true
		}
	case c == ":":
		g.capture = false
	case c == "#":
		skipComment(t, start)
		return nil, false, true
	case c == "=" || c == "!" || c == "<":
		return parseLookaround(t, st, verbose, nested, start, c), false, true
	case c == "(":
		return parseConditional(t, st, verbose, nested, start), false, true
	case c == ">":
		g.capture, g.atomic = false, true
	case len(char) == 1 && (flagOf(char[0]) != 0 || c == "-"):
		var isGlobal bool
		g.add, g.del, isGlobal = parseFlags(t, st, char[0])
		if isGlobal {
			if !first || nonEmpty {
				panic(t.error("global flags not at the start of the expression", t.tell()-start))
			}
			return nil, true, true
		}
		g.capture = false
	default:
		panic(t.error("unknown extension ?"+c, len(char)+1))
	}
	return nil, false, false
}

// parsePExtension reads what follows "(?P": "<name>" opens a named group (its name is
// returned), "=name)" is the reference to that group (ref).
func parsePExtension(t *tokenizer, st *state) (name string, ref *node) {
	switch {
	case t.match("<"):
		name = t.getuntil('>', "group name")
		t.checkgroupname(name, 1)
		return name, nil
	case t.match("="):
		name = t.getuntil(')', "group name")
		t.checkgroupname(name, 1)
		gid, ok := st.groupdict[name]
		nl := len([]rune(name))
		if !ok {
			panic(t.error("unknown group name "+pyRepr(name), nl+1))
		}
		if !st.checkgroup(gid) {
			panic(t.error("cannot refer to an open group", nl+1))
		}
		st.checklookbehindgroup(gid, t)
		return "", &node{kind: kGroupRef, ref: gid}
	}
	ch := t.get()
	if ch == nil {
		panic(t.error("unexpected end of pattern", 0))
	}
	panic(t.error("unknown extension ?P"+string(ch), len(ch)+2))
}

// skipComment skips a "(?#...)" comment up to its ")".
func skipComment(t *tokenizer, start int) {
	for {
		if t.next == nil {
			panic(t.error("missing ), unterminated comment", t.tell()-start))
		}
		if string(t.get()) == ")" {
			return
		}
	}
}

// parseLookaround parses a lookahead ("(?=", "(?!", c is "=" or "!") or a lookbehind
// ("(?<=", "(?<!", c is "<"): the outermost lookbehind records the groups opened before it
// (state.lookbehindgroups). An empty negative lookaround is FAILURE, as in _parse.
func parseLookaround(t *tokenizer, st *state, verbose bool, nested, start int, c string) *node {
	dir := 1
	saved := st.lookbehindgroups
	if c == "<" {
		ch := t.get()
		if ch == nil {
			panic(t.error("unexpected end of pattern", 0))
		}
		if c = string(ch); c != "=" && c != "!" {
			panic(t.error("unknown extension ?<"+c, len(ch)+2))
		}
		dir = -1
		if saved < 0 {
			st.lookbehindgroups = st.groups()
		}
	}
	p := parseSub(t, st, verbose, nested+1)
	if dir < 0 && saved < 0 {
		st.lookbehindgroups = -1
	}
	if !t.match(")") {
		panic(unterminatedGroup(t, start))
	}
	switch {
	case c == "=":
		return &node{kind: kAssert, dir: dir, sub: p}
	case len(p) > 0:
		return &node{kind: kAssertNot, dir: dir, sub: p}
	}
	return &node{kind: kFailure}
}

// parseGroupBody parses the group contents up to ")" (verbose as the group's flags set it)
// and closes a capturing group with its width.
func parseGroupBody(t *tokenizer, st *state, verbose bool, nested, start int, g groupSpec) *node {
	group := 0
	if g.capture {
		var msg string
		if group, msg = st.opengroup(g.name); msg != "" {
			panic(t.error(msg, len([]rune(g.name))+1))
		}
	}
	subVerbose := (verbose || g.add&VERBOSE != 0) && g.del&VERBOSE == 0
	p := parseSub(t, st, subVerbose, nested+1)
	if !t.match(")") {
		panic(unterminatedGroup(t, start))
	}
	if group > 0 {
		st.groupwidths[group] = width(p, st)
		st.closed[group] = true
	}
	if g.atomic {
		return &node{kind: kAtomic, sub: p}
	}
	return &node{kind: kGroup, group: group, add: g.add, del: g.del, sub: p}
}
