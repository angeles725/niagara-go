package pyjson

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Kind is the JSON value type, with Python's int/float split kept.
type Kind int

// JSON value kinds.
const (
	Null Kind = iota
	Bool
	Int
	Float
	Str
	Array
	Object
)

// Value is a decoded JSON value with Python json semantics: objects keep
// insertion order (dict), ints are arbitrary precision, strings are WTF-8.
type Value struct {
	Kind Kind
	B    bool
	I    *big.Int
	F    float64
	S    string
	A    []Value
	Keys []string
	M    map[string]Value
}

// String builds a str value.
func String(s string) Value { return Value{Kind: Str, S: s} }

// IntValue builds an int value.
func IntValue(n int64) Value { return Value{Kind: Int, I: big.NewInt(n)} }

// NewObject returns an empty dict.
func NewObject() Value { return Value{Kind: Object, M: map[string]Value{}} }

// Set assigns key like dict[key] = v (first insertion fixes the position).
func (v *Value) Set(key string, val Value) {
	if _, ok := v.M[key]; !ok {
		v.Keys = append(v.Keys, key)
	}
	v.M[key] = val
}

// Get mirrors dict.get(key): the value and whether the key exists.
func (v Value) Get(key string) (Value, bool) {
	if v.Kind != Object {
		return Value{}, false
	}
	x, ok := v.M[key]
	return x, ok
}

// Truthy mirrors bool(value).
func (v Value) Truthy() bool {
	switch v.Kind {
	case Bool:
		return v.B
	case Int:
		return v.I.Sign() != 0
	case Float:
		return v.F != 0
	case Str:
		return v.S != ""
	case Array:
		return len(v.A) > 0
	case Object:
		return len(v.Keys) > 0
	}
	return false
}

// Numeric returns the value as a number when Python treats it as one.
func (v Value) Numeric() (*big.Float, bool) {
	switch v.Kind {
	case Bool:
		if v.B {
			return big.NewFloat(1), true
		}
		return big.NewFloat(0), true
	case Int:
		return new(big.Float).SetInt(v.I), true
	case Float:
		if math.IsNaN(v.F) || math.IsInf(v.F, 0) {
			return nil, false
		}
		return big.NewFloat(v.F), true
	}
	return nil, false
}

// Equal mirrors Python == between JSON values (1 == 1.0 == True).
func Equal(a, b Value) bool {
	if na, ok := a.Numeric(); ok {
		if nb, ok := b.Numeric(); ok {
			return na.Cmp(nb) == 0
		}
		return false
	}
	if a.Kind == Float && b.Kind == Float {
		return a.F == b.F
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case Null:
		return true
	case Str:
		return a.S == b.S
	case Array:
		if len(a.A) != len(b.A) {
			return false
		}
		for i := range a.A {
			if !Equal(a.A[i], b.A[i]) {
				return false
			}
		}
		return true
	case Object:
		if len(a.Keys) != len(b.Keys) {
			return false
		}
		for _, k := range a.Keys {
			bv, ok := b.M[k]
			if !ok || !Equal(a.M[k], bv) {
				return false
			}
		}
		return true
	}
	return false
}

// TypeName is Python's type(v).__name__.
func (v Value) TypeName() string {
	return [...]string{"NoneType", "bool", "int", "float", "str", "list", "dict"}[v.Kind]
}

// decodeError mirrors json.JSONDecodeError's str().
func decodeError(msg string, doc []rune, pos int) error {
	line, last := 1, -1
	for i := 0; i < pos && i < len(doc); i++ {
		if doc[i] == '\n' {
			line++
			last = i
		}
	}
	return fmt.Errorf("%s: line %d column %d (char %d)", msg, line, pos-last, pos)
}

type stopIteration int

// MaxDepth is the deepest container nesting Loads accepts. CPython 3.15 has
// no fixed limit: its scanner raises RecursionError when the C stack runs out
// (about 52,100 levels on Linux's 8 MiB main-thread stack, fewer on smaller
// stacks). Loads stops at this fixed bound, below that ceiling, instead of
// overflowing Go's stack and killing the process.
const MaxDepth = 50000

// RecursionError is json.loads' RecursionError on runaway nesting. CPython
// 3.15's text carries the stack it used ("Stack overflow (used 8152 kB) while
// decoding ..."); this is the fixed text of the earlier depth-counted limit.
type RecursionError struct{ Container string } // "array" or "object"

func (e *RecursionError) Error() string {
	return "maximum recursion depth exceeded while decoding a JSON " + e.Container + " from a unicode string"
}

type jsonScanner struct {
	s     []rune
	depth int
}

// enter mirrors Py_EnterRecursiveCall in scan_once before a container.
func (p *jsonScanner) enter(container string) {
	if p.depth++; p.depth > MaxDepth {
		panic(jsonError{&RecursionError{container}})
	}
}

func isWS(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func (p *jsonScanner) skipWS(i int) int {
	for i < len(p.s) && isWS(p.s[i]) {
		i++
	}
	return i
}

func (p *jsonScanner) has(i int, lit string) bool {
	return i+len(lit) <= len(p.s) && string(p.s[i:i+len(lit)]) == lit
}

// Loads mirrors json.loads(str) with the C scanner.
func Loads(s []rune) (v Value, err error) {
	if len(s) > 0 && s[0] == 0xFEFF {
		return v, decodeError("Unexpected UTF-8 BOM (decode using utf-8-sig)", s, 0)
	}
	p := &jsonScanner{s: s}
	defer func() {
		if r := recover(); r != nil {
			if si, ok := r.(stopIteration); ok {
				err = decodeError("Expecting value", s, int(si))
				return
			}
			if e, ok := r.(jsonError); ok {
				err = e.err
				return
			}
			panic(r)
		}
	}()
	v, end := p.scan(p.skipWS(0))
	end = p.skipWS(end)
	if end != len(s) {
		return v, decodeError("Extra data", s, end)
	}
	return v, nil
}

type jsonError struct{ err error }

func (p *jsonScanner) fail(msg string, pos int) { panic(jsonError{decodeError(msg, p.s, pos)}) }

func (p *jsonScanner) scan(i int) (Value, int) {
	if i >= len(p.s) {
		panic(stopIteration(i))
	}
	switch c := p.s[i]; {
	case c == '"':
		s, end := p.scanString(i + 1)
		return String(s), end
	case c == '{':
		p.enter("object")
		v, end := p.scanObject(i + 1)
		p.depth--
		return v, end
	case c == '[':
		p.enter("array")
		v, end := p.scanArray(i + 1)
		p.depth--
		return v, end
	case c == 'n' && p.has(i, "null"):
		return Value{}, i + 4
	case c == 't' && p.has(i, "true"):
		return Value{Kind: Bool, B: true}, i + 4
	case c == 'f' && p.has(i, "false"):
		return Value{Kind: Bool}, i + 5
	case c == 'N' && p.has(i, "NaN"):
		return Value{Kind: Float, F: math.NaN()}, i + 3
	case c == 'I' && p.has(i, "Infinity"):
		return Value{Kind: Float, F: math.Inf(1)}, i + 8
	case c == '-' && p.has(i, "-Infinity"):
		return Value{Kind: Float, F: math.Inf(-1)}, i + 9
	}
	return p.scanNumber(i)
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func (p *jsonScanner) scanNumber(start int) (Value, int) {
	s, i := p.s, start
	if s[i] == '-' {
		i++
		if i >= len(s) {
			panic(stopIteration(start))
		}
	}
	switch {
	case s[i] >= '1' && s[i] <= '9':
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	case s[i] == '0':
		i++
	default:
		panic(stopIteration(start))
	}
	isFloat := false
	if i+1 < len(s) && s[i] == '.' && isDigit(s[i+1]) {
		isFloat = true
		i += 2
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i+1 < len(s) && (s[i] == 'e' || s[i] == 'E') {
		e := i
		i++
		if i+1 < len(s) && (s[i] == '-' || s[i] == '+') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		if isDigit(s[i-1]) {
			isFloat = true
		} else {
			i = e
		}
	}
	text := string(s[start:i])
	if isFloat {
		f, _ := strconv.ParseFloat(text, 64)
		return Value{Kind: Float, F: f}, i
	}
	n, _ := new(big.Int).SetString(text, 10)
	return Value{Kind: Int, I: n}, i
}

func hexVal(r rune) (rune, bool) {
	switch {
	case r >= '0' && r <= '9':
		return r - '0', true
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10, true
	case r >= 'A' && r <= 'F':
		return r - 'A' + 10, true
	}
	return 0, false
}

// scanString mirrors scanstring_unicode (strict=True); end is just past the quote.
func (p *jsonScanner) scanString(end int) (string, int) {
	s, begin := p.s, end-1
	var out []rune
	for {
		next := end
		for ; next < len(s); next++ {
			c := s[next]
			if c == '"' || c == '\\' {
				break
			}
			if c <= 0x1F {
				p.fail("Invalid control character at", next)
			}
		}
		if next >= len(s) {
			p.fail("Unterminated string starting at", begin)
		}
		out = append(out, s[end:next]...)
		next++
		if s[next-1] == '"' {
			return WTF8(out), next
		}
		if next == len(s) {
			p.fail("Unterminated string starting at", begin)
		}
		c := s[next]
		if c != 'u' {
			end = next + 1
			esc := map[rune]rune{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
			r, ok := esc[c]
			if !ok {
				p.fail("Invalid \\escape", end-2)
			}
			out = append(out, r)
			continue
		}
		next++
		end = next + 4
		if end >= len(s) {
			p.fail("Invalid \\uXXXX escape", next-1)
		}
		u := p.hex4(next, end)
		next = end
		if u >= 0xD800 && u <= 0xDBFF && end+6 < len(s) && s[next] == '\\' && s[next+1] == 'u' {
			end += 6
			u2 := p.hex4(next+2, end)
			if u2 >= 0xDC00 && u2 <= 0xDFFF {
				u = 0x10000 + (u-0xD800)<<10 + (u2 - 0xDC00)
			} else {
				end -= 6
			}
		}
		out = append(out, u)
	}
}

func (p *jsonScanner) hex4(from, end int) rune {
	var u rune
	for k := from; k < end; k++ {
		d, ok := hexVal(p.s[k])
		if !ok {
			p.fail("Invalid \\uXXXX escape", end-5)
		}
		u = u<<4 | d
	}
	return u
}

func (p *jsonScanner) scanObject(i int) (Value, int) {
	obj := NewObject()
	s := p.s
	i = p.skipWS(i)
	if i >= len(s) || s[i] != '}' {
		for {
			if i >= len(s) || s[i] != '"' {
				p.fail("Expecting property name enclosed in double quotes", i)
			}
			key, next := p.scanString(i + 1)
			i = p.skipWS(next)
			if i >= len(s) || s[i] != ':' {
				p.fail("Expecting ':' delimiter", i)
			}
			val, next := p.scan(p.skipWS(i + 1))
			obj.Set(key, val)
			i = p.skipWS(next)
			if i < len(s) && s[i] == '}' {
				break
			}
			if i >= len(s) || s[i] != ',' {
				p.fail("Expecting ',' delimiter", i)
			}
			comma := i
			i = p.skipWS(i + 1)
			if i < len(s) && s[i] == '}' {
				p.fail("Illegal trailing comma before end of object", comma)
			}
		}
	}
	return obj, i + 1
}

func (p *jsonScanner) scanArray(i int) (Value, int) {
	arr := Value{Kind: Array, A: []Value{}}
	s := p.s
	i = p.skipWS(i)
	if i >= len(s) || s[i] != ']' {
		for {
			val, next := p.scan(i)
			arr.A = append(arr.A, val)
			i = p.skipWS(next)
			if i < len(s) && s[i] == ']' {
				break
			}
			if i >= len(s) || s[i] != ',' {
				p.fail("Expecting ',' delimiter", i)
			}
			comma := i
			i = p.skipWS(i + 1)
			if i < len(s) && s[i] == ']' {
				p.fail("Illegal trailing comma before end of array", comma)
			}
		}
	}
	return arr, i + 1
}

// SortedKeys returns the keys in Python sorted() order (code point order).
func SortedKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.Compare(out[j-1], out[j]) > 0; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
