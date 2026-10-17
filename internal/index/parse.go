package index

import (
	"encoding/json"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// parser is a hand-written JSON decoder for large indexes. It follows the
// control flow of CPython's C scanner (Modules/_json.c) so that a syntax error
// is reported with the same message at the same position. Object keys, numbers
// and short strings are interned: an index repeats the same keys, line numbers
// and module names hundreds of thousands of times.
type parser struct {
	data []byte
	keys map[string]string
	err  *DecodeError

	// stack holds the members of the objects and arrays being parsed; each
	// finished container copies its run into a slice of exactly its size.
	stack []any
	kstk  []string
	// Direct-mapped caches: keys check hotK before the keys map; short string
	// values and numbers are shared only while they stay in hotV and hotN, so
	// unique values never grow a map.
	hotK [hotSize]string
	hotV [hotSize]any
	hotN [hotSize]any
}

// internMax is the longest string value that is shared through the hot cache.
const internMax = 24

func newParser(data []byte) *parser {
	return &parser{data: data, keys: map[string]string{}}
}

// fail records the first error (a Python message at byte offset pos).
func (p *parser) fail(msg string, pos int) {
	if p.err == nil {
		p.err = newDecodeError(p.data, msg, pos, false)
	}
}

func (p *parser) ws(i int) int {
	for i < len(p.data) {
		switch p.data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// document is json.loads(s): a BOM is an error, then one value and only
// whitespace after it.
func (p *parser) document() (any, error) {
	if len(p.data) >= 3 && p.data[0] == 0xEF && p.data[1] == 0xBB && p.data[2] == 0xBF {
		p.fail("Unexpected UTF-8 BOM (decode using utf-8-sig)", 0)
		return nil, p.err
	}
	i := p.ws(0)
	v, end, ok := p.value(i)
	if !ok {
		if p.err == nil {
			p.fail("Expecting value", i)
		}
		return nil, p.err
	}
	if end = p.ws(end); end != len(p.data) {
		p.fail("Extra data", end)
		return nil, p.err
	}
	return v, nil
}

// value is scan_once at i: the value and the offset after it. ok is false when
// no value starts at i (the caller reports "Expecting value") or after an error
// inside the value (p.err set).
func (p *parser) value(i int) (any, int, bool) {
	if i >= len(p.data) {
		return nil, i, false
	}
	switch c := p.data[i]; c {
	case '"':
		s, end, ok := p.str(i, false)
		return s, end, ok
	case '{':
		return p.object(i + 1)
	case '[':
		return p.array(i + 1)
	case 'n':
		if p.lit(i, "null") {
			return nil, i + 4, true
		}
	case 't':
		if p.lit(i, "true") {
			return true, i + 4, true
		}
	case 'f':
		if p.lit(i, "false") {
			return false, i + 5, true
		}
	case 'N':
		if p.lit(i, "NaN") {
			return json.Number("NaN"), i + 3, true
		}
	case 'I':
		if p.lit(i, "Infinity") {
			return json.Number("Infinity"), i + 8, true
		}
	case '-':
		if p.lit(i, "-Infinity") {
			return json.Number("-Infinity"), i + 9, true
		}
	}
	return p.number(i)
}

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

// object is _parse_object_unicode; i is just after '{'.
func (p *parser) object(i int) (any, int, bool) {
	base := len(p.stack)
	defer func() { p.stack, p.kstk = p.stack[:base], p.kstk[:base] }()
	i = p.ws(i)
	if i < len(p.data) && p.data[i] == '}' {
		return &Object{}, i + 1, true
	}
	for {
		if i >= len(p.data) || p.data[i] != '"' {
			p.fail("Expecting property name enclosed in double quotes", i)
			return nil, i, false
		}
		k, end, ok := p.str(i, true)
		if !ok {
			return nil, end, false
		}
		i = p.ws(end)
		if i >= len(p.data) || p.data[i] != ':' {
			p.fail("Expecting ':' delimiter", i)
			return nil, i, false
		}
		i = p.ws(i + 1)
		v, end, ok := p.value(i)
		if !ok {
			p.fail("Expecting value", i)
			return nil, end, false
		}
		p.kstk = append(p.kstk, k.(string))
		p.stack = append(p.stack, v)
		i = p.ws(end)
		if i < len(p.data) && p.data[i] == '}' {
			return newObjectFrom(p.kstk[base:], p.stack[base:]), i + 1, true
		}
		if i >= len(p.data) || p.data[i] != ',' {
			p.fail("Expecting ',' delimiter", i)
			return nil, i, false
		}
		comma := i
		i = p.ws(i + 1)
		if i < len(p.data) && p.data[i] == '}' {
			p.fail("Illegal trailing comma before end of object", comma)
			return nil, i, false
		}
	}
}

// array is _parse_array_unicode; i is just after '['.
func (p *parser) array(i int) (any, int, bool) {
	base := len(p.stack)
	defer func() { p.stack, p.kstk = p.stack[:base], p.kstk[:base] }()
	i = p.ws(i)
	if i < len(p.data) && p.data[i] == ']' {
		return []any{}, i + 1, true
	}
	for {
		v, end, ok := p.value(i)
		if !ok {
			p.fail("Expecting value", i)
			return nil, end, false
		}
		p.stack = append(p.stack, v)
		p.kstk = append(p.kstk, "")
		i = p.ws(end)
		if i < len(p.data) && p.data[i] == ']' {
			return append([]any(nil), p.stack[base:]...), i + 1, true
		}
		if i >= len(p.data) || p.data[i] != ',' {
			p.fail("Expecting ',' delimiter", i)
			return nil, i, false
		}
		comma := i
		i = p.ws(i + 1)
		if i < len(p.data) && p.data[i] == ']' {
			p.fail("Illegal trailing comma before end of array", comma)
			return nil, i, false
		}
	}
}

// newObjectFrom builds an object from parsed members in order, with Python's
// rule for a repeated key (first position, last value).
func newObjectFrom(keys []string, vals []any) *Object {
	o := &Object{keys: append([]string(nil), keys...), vals: append([]any(nil), vals...)}
	if len(keys) > indexFrom {
		o.idx = make(map[string]int, len(keys))
	}
	for i, k := range keys {
		if j := o.findIn(k, i); j >= 0 {
			return dedupe(keys, vals)
		}
		if o.idx != nil {
			o.idx[k] = i
		}
	}
	return o
}

// findIn is find over the first n keys while the object is being built.
func (o *Object) findIn(k string, n int) int {
	if o.idx != nil {
		if j, ok := o.idx[k]; ok {
			return j
		}
		return -1
	}
	for j, key := range o.keys[:n] {
		if key == k {
			return j
		}
	}
	return -1
}

// dedupe is the slow path for an object with a repeated key.
func dedupe(keys []string, vals []any) *Object {
	o := NewObject()
	for i, k := range keys {
		o.Set(k, vals[i])
	}
	return o
}
