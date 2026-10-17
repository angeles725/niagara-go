package help

// fnmatch.fnmatch on POSIX (case-sensitive, os.path.normcase is the
// identity), following Python 3.15 fnmatch._translate: "*" matches any run
// of characters including "/" and newlines, "?" any one character, and
// "[...]" a set with "!" negation and ranges, where an empty range (z-a)
// removes both of its endpoints and an unclosed "[" is a literal.
// Matching works on code points with a direct wildcard matcher instead of a
// regular expression: Python's atomic groups do not change which names a
// shell pattern matches, and Go's regexp would read "[:alpha:]" inside a set.
//
// TODO(fnmatch): move to the shared internal/fnmatch package once it is on main.

type globKind uint8

const (
	globLiteral globKind = iota
	globAny              // ?
	globStar             // *
	globSet              // [...]
	globNever            // an empty set: matches nothing
)

type runeRange struct{ lo, hi rune }

type globTok struct {
	kind   globKind
	r      rune
	negate bool
	set    []runeRange
}

func (t globTok) matches(c rune) bool {
	switch t.kind {
	case globLiteral:
		return c == t.r
	case globAny:
		return true
	case globSet:
		in := false
		for _, rg := range t.set {
			if rg.lo <= c && c <= rg.hi {
				in = true
				break
			}
		}
		return in != t.negate
	}
	return false
}

// fnmatch reports whether name matches the shell pattern pat.
func fnmatch(name, pat string) bool { return globMatch([]rune(name), compileGlob(pat)) }

func compileGlob(pattern string) []globTok {
	pat := []rune(pattern)
	var toks []globTok
	n := len(pat)
	for i := 0; i < n; {
		c := pat[i]
		i++
		switch c {
		case '*':
			toks = append(toks, globTok{kind: globStar})
			for i < n && pat[i] == '*' {
				i++
			}
		case '?':
			toks = append(toks, globTok{kind: globAny})
		case '[':
			j := i
			if j < n && pat[j] == '!' {
				j++
			}
			if j < n && pat[j] == ']' {
				j++
			}
			for j < n && pat[j] != ']' {
				j++
			}
			if j >= n {
				toks = append(toks, globTok{kind: globLiteral, r: '['})
				continue
			}
			toks = append(toks, globSetTok(pat, i, j))
			i = j + 1
		default:
			toks = append(toks, globTok{kind: globLiteral, r: c})
		}
	}
	return toks
}

// globSetTok builds the set pat[i:j] (between "[" and "]") the way
// _translate splits it into range chunks.
func globSetTok(pat []rune, i, j int) globTok {
	stuff := pat[i:j]
	var chunks [][]rune
	if !containsRune(stuff, '-') {
		chunks = [][]rune{append([]rune(nil), stuff...)}
	} else {
		k := i + 1
		if pat[i] == '!' {
			k = i + 2
		}
		for {
			k = indexRune(pat, '-', k, j)
			if k < 0 {
				break
			}
			chunks = append(chunks, append([]rune(nil), pat[i:k]...))
			i = k + 1
			k += 3
		}
		if chunk := pat[i:j]; len(chunk) > 0 {
			chunks = append(chunks, append([]rune(nil), chunk...))
		} else {
			chunks[len(chunks)-1] = append(chunks[len(chunks)-1], '-')
		}
		for k := len(chunks) - 1; k > 0; k-- { // remove empty ranges
			prev, cur := chunks[k-1], chunks[k]
			if prev[len(prev)-1] > cur[0] {
				chunks[k-1] = append(append([]rune(nil), prev[:len(prev)-1]...), cur[1:]...)
				chunks = append(chunks[:k], chunks[k+1:]...)
			}
		}
	}
	total := 0
	for _, ch := range chunks {
		total += len(ch)
	}
	if total == 0 {
		return globTok{kind: globNever}
	}
	tok := globTok{kind: globSet}
	if chunks[0][0] == '!' { // "[!]" alone (stuff == "!") negates nothing: any character
		tok.negate = true
		chunks[0] = chunks[0][1:]
	}
	// Adjacent chunks are joined by a range: the last character of one and
	// the first of the next are its ends; every other character is literal.
	last := len(chunks) - 1
	for k, ch := range chunks {
		lo, hi := 0, len(ch)
		if k > 0 {
			lo = 1
		}
		if k < last && len(ch) > 0 {
			hi--
			tok.set = append(tok.set, runeRange{ch[len(ch)-1], chunks[k+1][0]})
		}
		for _, c := range ch[min(lo, hi):hi] {
			tok.set = append(tok.set, runeRange{c, c})
		}
	}
	return tok
}

func containsRune(rs []rune, r rune) bool { return indexRune(rs, r, 0, len(rs)) >= 0 }

// indexRune is str.find(r, start, end) over runes.
func indexRune(rs []rune, r rune, start, end int) int {
	for k := start; k < end && k < len(rs); k++ {
		if rs[k] == r {
			return k
		}
	}
	return -1
}

// globMatch is the classic single-backtrack wildcard match: every token but
// "*" has width one, so retrying only the last star is exhaustive.
func globMatch(name []rune, toks []globTok) bool {
	ni, ti := 0, 0
	star, mark := -1, 0
	for ni < len(name) {
		switch {
		case ti < len(toks) && toks[ti].kind == globStar:
			star, mark = ti, ni
			ti++
		case ti < len(toks) && toks[ti].matches(name[ni]):
			ni++
			ti++
		case star >= 0:
			mark++
			ni, ti = mark, star+1
		default:
			return false
		}
	}
	for ti < len(toks) && toks[ti].kind == globStar {
		ti++
	}
	return ti == len(toks)
}

// lowerGlob is fnmatch(name.lower(), pat.lower()).
func lowerGlob(name, pat string) bool { return fnmatch(lowerStr(name), lowerStr(pat)) }
