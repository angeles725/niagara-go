// Package fnmatch ports Python 3.15 fnmatch matching: Fnmatch and FnmatchCase.
//
// Fnmatch applies os.path.normcase on POSIX, which is the identity, so it is case-sensitive
// like FnmatchCase. On Windows Python's ntpath.normcase lowercases both strings and turns
// '/' into '\': a Windows Python tool matches case-insensitively where this package does
// not; tools that need that lower both sides themselves (as the niagara-help and
// module-navigator searches already do).
//
// Matching follows fnmatch.translate part by part but writes RE2 text: RE2 has no atomic
// groups or (?!), so those become a lazy group and an empty class.
package fnmatch

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// part is one translated pattern element: the RE2 equivalent of Python's regex text.
type part struct {
	re2  string
	star bool
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
			res = append(res, part{re2: ".*", star: true})
			for i < n && pat[i] == '*' {
				i++
			}
		case '?':
			res = append(res, part{re2: "."})
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
				res = append(res, part{re2: `\[`})
				continue
			}
			stuff := setStuff(pat, i, j)
			i = j + 1
			switch {
			case stuff == "":
				res = append(res, part{re2: `[^\x{0}-\x{10FFFF}]`})
			case stuff == "!":
				res = append(res, part{re2: "."})
			default:
				stuff = setOps.ReplaceAllString(stuff, `\$1`)
				if stuff[0] == '!' {
					stuff = "^" + stuff[1:]
				} else if stuff[0] == '^' || stuff[0] == '[' {
					stuff = `\` + stuff
				}
				res = append(res, part{re2: re2Class(stuff)})
			}
		default:
			res = append(res, part{re2: regexp.QuoteMeta(string(c))})
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

// join is _join_translated_parts in RE2: fixed parts before the first star, a lazy
// (?:.*?...) group between stars where Python uses the atomic (?>.*?...), .* and the tail
// after the last.
func join(parts []part) string {
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
			b.WriteString(p.re2)
		}
	} else {
		for _, p := range parts[:stars[0]] {
			b.WriteString(p.re2)
		}
		i := stars[0] + 1
		for _, j := range stars[1:] {
			b.WriteString("(?:.*?")
			for _, p := range parts[i:j] {
				b.WriteString(p.re2)
			}
			b.WriteByte(')')
			i = j + 1
		}
		b.WriteString(".*")
		for _, p := range parts[i:] {
			b.WriteString(p.re2)
		}
	}
	b.WriteString(`)\z`)
	return b.String()
}

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
	re := regexp.MustCompile(`^` + join(translateParts(pat)))
	cache[pat] = re
	return re
}

// FnmatchCase is fnmatch.fnmatchcase(name, pat): case-sensitive shell-style matching.
func FnmatchCase(name, pat string) bool { return compile(pat).MatchString(name) }

// Fnmatch is fnmatch.fnmatch(name, pat) with POSIX os.path.normcase (see the package doc).
func Fnmatch(name, pat string) bool { return FnmatchCase(name, pat) }
