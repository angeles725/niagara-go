package pyre

// width is SubPattern.getwidth: the (min, max) number of characters a sequence matches.
func width(seq []*node, st *state) [2]int {
	lo, hi := 0, 0
	for _, n := range seq {
		var w [2]int
		switch n.kind {
		case kBranch:
			w = [2]int{maxWidth, 0}
			for _, b := range n.branches {
				bw := width(b, st)
				w[0], w[1] = min(w[0], bw[0]), max(w[1], bw[1])
			}
		case kAtomic, kGroup:
			w = width(n.sub, st)
		case kRepeat:
			iw := width(n.sub, st)
			lo = satAdd(lo, satMul(iw[0], n.min))
			if n.max == maxRepeat && iw[1] > 0 {
				hi = maxWidth
			} else {
				hi = satAdd(hi, satMul(iw[1], n.max))
			}
			continue
		case kLiteral, kNotLiteral, kAny, kIn:
			w = [2]int{1, 1}
		case kGroupRef:
			w = st.groupwidths[n.ref]
		case kGroupRefExists:
			w = width(n.yes, st)
			if n.hasNo {
				nw := width(n.no, st)
				w[0], w[1] = min(w[0], nw[0]), max(w[1], nw[1])
			} else {
				w[0] = 0
			}
		}
		lo, hi = satAdd(lo, w[0]), satAdd(hi, w[1])
	}
	return [2]int{min(lo, maxWidth), min(hi, maxWidth)}
}

func satAdd(a, b int) int { return min(a+b, maxWidth) }

func satMul(a, b int) int {
	if a != 0 && b > maxWidth/a {
		return maxWidth
	}
	return a * b
}

// checkCompile runs the checks re._compiler makes after parsing: a lookbehind must
// match a fixed number of characters.
func checkCompile(seq []*node, st *state) *Error {
	for _, n := range seq {
		if (n.kind == kAssert || n.kind == kAssertNot) && n.dir < 0 {
			w := width(n.sub, st)
			if w[0] > 0xFFFFFFFF {
				return newError("looks too much behind", "", -1)
			}
			if w[0] != w[1] {
				return newError("look-behind requires fixed-width pattern", "", -1)
			}
		}
		for _, sub := range append([][]*node{n.sub, n.yes, n.no}, n.branches...) {
			if err := checkCompile(sub, st); err != nil {
				return err
			}
		}
	}
	return nil
}

// validate parses a pattern and runs the compile checks: the re.error re.compile raises.
func validate(pattern string, flags Flags) ([]*node, *state, error) {
	seq, st, err := parse(pattern, flags)
	if err != nil {
		return nil, nil, err
	}
	if e := checkCompile(seq, st); e != nil {
		return nil, nil, e
	}
	return seq, st, nil
}
