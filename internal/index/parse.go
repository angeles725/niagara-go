package index

import (
	"encoding/json"
	"strconv"
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

	// depth is the number of objects and arrays open; past MaxDepth the parse
	// stops with a RecursionError instead of growing the Go stack.
	depth int
	deep  *RecursionError
}

// MaxDepth is the deepest nesting of objects and arrays a document may have
// (encoding/json's limit). CPython stops with a RecursionError ("Stack
// overflow (used N kB) while decoding a JSON array ...") at a depth that
// depends on its C stack; the port reports the fixed limit instead.
const MaxDepth = 10000

// RecursionError is the error of a document nested deeper than MaxDepth.
type RecursionError struct{ Container string }

func (e *RecursionError) Error() string {
	return "maximum nesting depth (" + strconv.Itoa(MaxDepth) + ") exceeded while decoding a JSON " +
		e.Container + " from a unicode string"
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
	if p.deep != nil {
		return nil, p.deep
	}
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
	case '{', '[':
		if p.depth >= MaxDepth {
			container := map[byte]string{'{': "object", '[': "array"}[c]
			p.deep = &RecursionError{container}
			p.fail("nesting too deep", i) // stops every enclosing container
			return nil, i, false
		}
		p.depth++
		var v any
		var end int
		var ok bool
		if c == '{' {
			v, end, ok = p.object(i + 1)
		} else {
			v, end, ok = p.array(i + 1)
		}
		p.depth--
		return v, end, ok
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
