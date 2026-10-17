package stationmodules

import (
	"sort"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// isWord is Python's \w on str: '_' and str.isalnum() characters.
func isWord(r rune) bool { return pycompat.IsWordRune(r) }

func isQuote(c byte) bool { return c == '"' || c == '\'' }

// stationRefs is station_refs: the alias -> module map of every m="a=mod ..." declaration
// and the {module: types} referenced by t="alias:Type", every declared module included.
// It reproduces re.findall(r'\bm=["\']([^"\']+)["\']'), then (\w+)=(\w+) inside each
// declaration, and t=["\']([A-Za-z0-9]+):([A-Za-z0-9_]+)["\'].
func stationRefs(xml string) (map[string]string, map[string]map[string]bool) {
	alias := map[string]string{}
	for i := 0; i+2 < len(xml); i++ {
		if xml[i] != 'm' || xml[i+1] != '=' || !isQuote(xml[i+2]) {
			continue
		}
		if prev, _ := utf8.DecodeLastRuneInString(xml[:i]); i > 0 && isWord(prev) {
			continue
		}
		j := i + 3
		for j < len(xml) && !isQuote(xml[j]) {
			j++
		}
		if j == i+3 || j == len(xml) {
			continue
		}
		for _, kv := range wordPairs(xml[i+3 : j]) {
			alias[kv[0]] = kv[1]
		}
		i = j
	}
	types := map[string]map[string]bool{}
	for i := 0; i+2 < len(xml); i++ {
		if xml[i] != 't' || xml[i+1] != '=' || !isQuote(xml[i+2]) {
			continue
		}
		a := i + 3
		e := a
		for e < len(xml) && isAlnumASCII(xml[e]) {
			e++
		}
		if e == a || e == len(xml) || xml[e] != ':' {
			continue
		}
		n := e + 1
		end := n
		for end < len(xml) && (isAlnumASCII(xml[end]) || xml[end] == '_') {
			end++
		}
		if end == n || end == len(xml) || !isQuote(xml[end]) {
			continue
		}
		if mod, ok := alias[xml[a:e]]; ok {
			if types[mod] == nil {
				types[mod] = map[string]bool{}
			}
			types[mod][xml[n:end]] = true
		}
		i = end
	}
	for _, mod := range alias {
		if types[mod] == nil {
			types[mod] = map[string]bool{}
		}
	}
	return alias, types
}

func isAlnumASCII(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// wordPairs is re.findall(r'(\w+)=(\w+)', s).
func wordPairs(s string) [][2]string {
	var out [][2]string
	rs := []rune(s)
	run := func(i int) int {
		for i < len(rs) && isWord(rs[i]) {
			i++
		}
		return i
	}
	for i := 0; i < len(rs); {
		e := run(i)
		if e == i {
			i++
			continue
		}
		if e < len(rs) && rs[e] == '=' {
			if v := run(e + 1); v > e+1 {
				out = append(out, [2]string{string(rs[i:e]), string(rs[e+1 : v])})
				i = v
				continue
			}
		}
		i = e
	}
	return out
}

// sortedKeys returns the keys of a set in sorted order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
