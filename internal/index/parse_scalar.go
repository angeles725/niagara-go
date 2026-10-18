package index

import (
	"encoding/json"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// The scalar half of the parser: literals, numbers and strings, with the hot
// caches that intern keys, short string values and numbers. parse.go holds the
// document, value dispatch, objects and arrays.

func (p *parser) lit(i int, word string) bool {
	return len(p.data)-i >= len(word) && string(p.data[i:i+len(word)]) == word
}

// number is _match_number_unicode: -?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?
func (p *parser) number(start int) (any, int, bool) {
	d, i := p.data, start
	if i < len(d) && d[i] == '-' {
		i++
	}
	switch {
	case i < len(d) && d[i] >= '1' && d[i] <= '9':
		for i++; i < len(d) && isDigit(d[i]); i++ {
		}
	case i < len(d) && d[i] == '0':
		i++
	default:
		return nil, start, false
	}
	if i+1 < len(d) && d[i] == '.' && isDigit(d[i+1]) {
		for i += 2; i < len(d) && isDigit(d[i]); i++ {
		}
	}
	if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
		j := i + 1
		if j < len(d) && (d[j] == '-' || d[j] == '+') {
			j++
		}
		k := j
		for k < len(d) && isDigit(d[k]) {
			k++
		}
		if k > j {
			i = k
		}
	}
	text := d[start:i]
	h := slot(text)
	if v := p.hotN[h]; v != nil && string(v.(json.Number)) == string(text) {
		return v, i, true
	}
	var v any = json.Number(text)
	p.hotN[h] = v
	return v, i, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// str is scanstring_unicode (strict) for the string whose quote is at q. A key
// is interned in p.keys, a short value in p.vals.
func (p *parser) str(q int, key bool) (any, int, bool) {
	d := p.data
	i := q + 1
	for i < len(d) {
		c := d[i]
		if c == '"' {
			return p.intern(d[q+1:i], key), i + 1, true
		}
		if c == '\\' {
			return p.escaped(q, i, key)
		}
		if c < 0x20 {
			p.fail("Invalid control character at", i)
			return nil, i, false
		}
		i++
	}
	p.fail("Unterminated string starting at", q)
	return nil, i, false
}

func (p *parser) intern(b []byte, key bool) any {
	if key {
		return p.key(b)
	}
	if len(b) > internMax {
		return string(b)
	}
	h := slot(b)
	if v := p.hotV[h]; v != nil && v.(string) == string(b) {
		return v
	}
	var v any = string(b)
	p.hotV[h] = v
	return v
}

// key returns the interned object key spelled b.
func (p *parser) key(b []byte) string {
	h := slot(b)
	if s := p.hotK[h]; s == string(b) && (len(b) > 0 || s == "") {
		return s
	}
	s, ok := p.keys[string(b)]
	if !ok {
		s = string(b)
		p.keys[s] = s
	}
	p.hotK[h] = s
	return s
}

const hotSize = 4096

// slot hashes a short byte string (FNV-1a) into the hot caches.
func slot(b []byte) uint32 {
	h := uint32(2166136261)
	for _, c := range b {
		h = (h ^ uint32(c)) * 16777619
	}
	return h % hotSize
}

// escaped finishes a string with escapes; i is the first backslash.
func (p *parser) escaped(q, i int, key bool) (any, int, bool) {
	d := p.data
	buf := append([]byte(nil), d[q+1:i]...)
	for i < len(d) {
		c := d[i]
		switch {
		case c == '"':
			return p.intern(buf, key), i + 1, true
		case c < 0x20:
			p.fail("Invalid control character at", i)
			return nil, i, false
		case c != '\\':
			buf = append(buf, c)
			i++
			continue
		}
		if i+1 >= len(d) {
			break
		}
		e := d[i+1]
		if e != 'u' {
			r, ok := simpleEscape(e)
			if !ok {
				p.fail("Invalid \\escape", i)
				return nil, i, false
			}
			buf = append(buf, r)
			i += 2
			continue
		}
		r, ok := hex4(d, i+2)
		if !ok {
			p.fail("Invalid \\uXXXX escape", i+1)
			return nil, i, false
		}
		i += 6
		if utf16.IsSurrogate(r) && r < 0xDC00 && i+6 < len(d) && d[i] == '\\' && d[i+1] == 'u' {
			lo, ok := hex4(d, i+2)
			if !ok {
				p.fail("Invalid \\uXXXX escape", i+1)
				return nil, i, false
			}
			if lo >= 0xDC00 && lo <= 0xDFFF {
				r = utf16.DecodeRune(r, lo)
				i += 6
			}
		}
		buf = utf8.AppendRune(buf, r) // a lone surrogate becomes U+FFFD
	}
	p.fail("Unterminated string starting at", q)
	return nil, i, false
}

func simpleEscape(e byte) (byte, bool) {
	switch e {
	case '"', '\\', '/':
		return e, true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	}
	return 0, false
}

func hex4(d []byte, i int) (rune, bool) {
	if i+4 > len(d) {
		return 0, false
	}
	for _, c := range d[i : i+4] {
		if !isHex(c) {
			return 0, false
		}
	}
	n, _ := strconv.ParseUint(string(d[i:i+4]), 16, 32)
	return rune(n), true
}
