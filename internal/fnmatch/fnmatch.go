// Package fnmatch ports Python 3.15 fnmatch: Fnmatch, FnmatchCase, Filter and Translate.
//
// Fnmatch applies os.path.normcase on POSIX, which is the identity, so it is case-sensitive
// like FnmatchCase. On Windows Python's ntpath.normcase lowercases both strings and turns
// '/' into '\': a Windows Python tool matches case-insensitively where this package does
// not; tools that need that lower both sides themselves (as the niagara-help and
// module-navigator searches already do).
//
// Translate returns Python's regular expression text byte for byte. Matching does not run
// that text: it uses an equivalent RE2 expression (RE2 has no atomic groups or (?!)).
package fnmatch

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// part is one translated pattern element: its Python regex text and its RE2 equivalent.
type part struct {
	py, re2 string
	star    bool
}

// reEscape is re.escape for one character.
func reEscape(c rune) string {
	if strings.ContainsRune("()[]{}?*+-|^$\\.&~# \t\n\r\v\f", c) {
		return `\` + string(c)
	}
	return string(c)
}

// translateParts is fnmatch._translate(pat, '*', '.').
func translateParts(pattern string) []part {
	pat := []rune(pattern)
	n := len(pat)
	var res []part
	for i := 0; i < n; {
		c := pat[i]
		i++
		switch c {
		case '*':
			res = append(res, part{py: ".*", re2: ".*", star: true})
			for i < n && pat[i] == '*' {
				i++
			}
		case '?':
			res = append(res, part{py: ".", re2: "."})
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
				res = append(res, part{py: `\[`, re2: `\[`})
				continue
			}
			stuff := setStuff(pat, i, j)
			i = j + 1
			switch {
			case stuff == "":
				res = append(res, part{py: "(?!)", re2: `[^\x{0}-\x{10FFFF}]`})
			case stuff == "!":
				res = append(res, part{py: ".", re2: "."})
			default:
				stuff = setOps.ReplaceAllString(stuff, `\$1`)
				if stuff[0] == '!' {
					stuff = "^" + stuff[1:]
				} else if stuff[0] == '^' || stuff[0] == '[' {
					stuff = `\` + stuff
				}
				res = append(res, part{py: "[" + stuff + "]", re2: re2Class(stuff)})
			}
		default:
			res = append(res, part{py: reEscape(c), re2: regexp.QuoteMeta(string(c))})
		}
	}
	return res
}

var setOps = regexp.MustCompile(`([&~|])`)

// setStuff is the body of a [...] set at pat[i:j], with fnmatch's range clean-up.
func setStuff(pat []rune, i, j int) string {
	if !containsRune(pat[i:j], '-') {
		return strings.ReplaceAll(string(pat[i:j]), `\`, `\\`)
	}
	var chunks []string
	k := i + 1
	if pat[i] == '!' {
		k = i + 2
	}
	for {
		k = indexRune(pat, '-', k, j)
		if k < 0 {
			break
		}
		chunks = append(chunks, string(pat[i:k]))
		i = k + 1
		k += 3
	}
	if chunk := string(pat[i:j]); chunk != "" {
		chunks = append(chunks, chunk)
	} else {
		chunks[len(chunks)-1] += "-"
	}
	// remove empty ranges: a chunk ending above the next chunk's first character
	for k := len(chunks) - 1; k > 0; k-- {
		prev, next := []rune(chunks[k-1]), []rune(chunks[k])
		if prev[len(prev)-1] > next[0] {
			chunks[k-1] = string(prev[:len(prev)-1]) + string(next[1:])
			chunks = append(chunks[:k], chunks[k+1:]...)
		}
	}
	for k, s := range chunks {
		chunks[k] = strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "-", `\-`)
	}
	return strings.Join(chunks, "-")
}

func containsRune(rs []rune, r rune) bool { return indexRune(rs, r, 0, len(rs)) >= 0 }

// indexRune is str.find(r, from, to) over runes.
func indexRune(rs []rune, r rune, from, to int) int {
	for k := max(from, 0); k < to && k < len(rs); k++ {
		if rs[k] == r {
			return k
		}
	}
	return -1
}

// re2Class turns a Python character class body into an RE2 class with every member written
// as \x{...}, so no character keeps a meaning RE2 gives it and Python does not ([:alpha:]).
func re2Class(stuff string) string {
	s := []rune(stuff)
	var b strings.Builder
	b.WriteByte('[')
	k := 0
	if s[0] == '^' {
		b.WriteByte('^')
		k = 1
	}
	item := func() rune {
		if s[k] == '\\' && k+1 < len(s) {
			k += 2
			return s[k-1]
		}
		k++
		return s[k-1]
	}
	for k < len(s) {
		lo := item()
		if k+1 < len(s) && s[k] == '-' {
			k++
			hi := item()
			fmt.Fprintf(&b, `\x{%x}-\x{%x}`, lo, hi)
			continue
		}
		fmt.Fprintf(&b, `\x{%x}`, lo)
	}
	b.WriteByte(']')
	return b.String()
}

// join is _join_translated_parts: fixed parts before the first star, (?>.*?...) between
// stars, .* and the tail after the last; RE2 uses a plain lazy group for the atomic one.
func join(parts []part, re2 bool) string {
	text := func(p part) string {
		if re2 {
			return p.re2
		}
		return p.py
	}
	var b strings.Builder
	b.WriteString("(?s:")
	var stars []int
	for i, p := range parts {
		if p.star {
			stars = append(stars, i)
		}
	}
	if len(stars) == 0 {
		for _, p := range parts {
			b.WriteString(text(p))
		}
	} else {
		for _, p := range parts[:stars[0]] {
			b.WriteString(text(p))
		}
		i := stars[0] + 1
		group := "(?>.*?"
		if re2 {
			group = "(?:.*?"
		}
		for _, j := range stars[1:] {
			b.WriteString(group)
			for _, p := range parts[i:j] {
				b.WriteString(text(p))
			}
			b.WriteByte(')')
			i = j + 1
		}
		b.WriteString(".*")
		for _, p := range parts[i:] {
			b.WriteString(text(p))
		}
	}
	b.WriteString(`)\z`)
	return b.String()
}

// Translate is fnmatch.translate(pat): Python's regular expression text.
func Translate(pat string) string { return join(translateParts(pat), false) }

var (
	cacheMu sync.Mutex
	cache   = map[string]*regexp.Regexp{}
)

const cacheSize = 32768 // functools.lru_cache(maxsize=32768) in _compile_pattern

func compile(pat string) *regexp.Regexp {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if re, ok := cache[pat]; ok {
		return re
	}
	if len(cache) >= cacheSize {
		clear(cache)
	}
	re := regexp.MustCompile(`^` + join(translateParts(pat), true))
	cache[pat] = re
	return re
}

// FnmatchCase is fnmatch.fnmatchcase(name, pat): case-sensitive shell-style matching.
func FnmatchCase(name, pat string) bool { return compile(pat).MatchString(name) }

// Fnmatch is fnmatch.fnmatch(name, pat) with POSIX os.path.normcase (see the package doc).
func Fnmatch(name, pat string) bool { return FnmatchCase(name, pat) }

// Filter is fnmatch.filter(names, pat): the names that match, in order.
func Filter(names []string, pat string) []string {
	re := compile(pat)
	var out []string
	for _, n := range names {
		if re.MatchString(n) {
			out = append(out, n)
		}
	}
	return out
}
