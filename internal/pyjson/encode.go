package pyjson

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// DumpOpts mirrors the json.dumps keyword arguments the tools use.
type DumpOpts struct {
	SortKeys    bool
	EnsureASCII bool
	Indent      int    // -1 = None
	ItemSep     string // separators[0]
	KeySep      string // separators[1]
}

// Canonical is json.dumps(sort_keys=True, separators=(",", ":"), ensure_ascii=True).
var Canonical = DumpOpts{SortKeys: true, EnsureASCII: true, Indent: -1, ItemSep: ",", KeySep: ":"}

// Dumps mirrors json.dumps(obj, **opts).
func Dumps(v Value, o DumpOpts) string {
	var b strings.Builder
	dumpTo(&b, v, o, 0)
	return b.String()
}

func dumpTo(b *strings.Builder, v Value, o DumpOpts, level int) {
	switch v.Kind {
	case Null:
		b.WriteString("null")
	case Bool:
		if v.B {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case Int:
		b.WriteString(v.I.String())
	case Float:
		b.WriteString(pycompat.JSONFloat(v.F))
	case Str:
		writeString(b, v.S, o.EnsureASCII)
	case Array:
		if len(v.A) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		newline(b, o, level+1)
		for i, x := range v.A {
			if i > 0 {
				b.WriteString(o.ItemSep)
				newline(b, o, level+1)
			}
			dumpTo(b, x, o, level+1)
		}
		newline(b, o, level)
		b.WriteByte(']')
	case Object:
		if len(v.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		keys := v.Keys
		if o.SortKeys {
			// Byte order, as SortedKeys and Python's str ordering of WTF-8 text.
			keys = slices.Clone(keys)
			slices.Sort(keys)
		}
		b.WriteByte('{')
		newline(b, o, level+1)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(o.ItemSep)
				newline(b, o, level+1)
			}
			writeString(b, k, o.EnsureASCII)
			b.WriteString(o.KeySep)
			dumpTo(b, v.M[k], o, level+1)
		}
		newline(b, o, level)
		b.WriteByte('}')
	}
}

const spaces = "                                                                "

// newline writes the line break and indentation of nesting level when o is indented.
func newline(b *strings.Builder, o DumpOpts, level int) {
	if o.Indent < 0 {
		return
	}
	b.WriteByte('\n')
	for n := o.Indent * level; n > 0; n -= len(spaces) {
		b.WriteString(spaces[:min(n, len(spaces))])
	}
}

// StrRepr mirrors repr(str). Valid UTF-8 is pycompat.StrRepr; a WTF-8 string holding
// lone surrogates (which pycompat.StrRepr would read as U+FFFD) is quoted rune by rune
// with the same rules, each surrogate as \udXXX.
func StrRepr(s string) string {
	if utf8.ValidString(s) {
		return pycompat.StrRepr(s)
	}
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range Runes(s) {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r >= 0xD800 && r <= 0xDFFF:
			fmt.Fprintf(&b, `\u%04x`, r)
		case pycompat.IsPrintable(r):
			b.WriteRune(r)
		case r <= 0xFF:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// Repr mirrors repr() of the decoded JSON value.
func (v Value) Repr() string {
	switch v.Kind {
	case Null:
		return "None"
	case Bool:
		return map[bool]string{true: "True", false: "False"}[v.B]
	case Int:
		return v.I.String()
	case Float:
		return pycompat.FloatRepr(v.F)
	case Str:
		return StrRepr(v.S)
	case Array:
		parts := make([]string, len(v.A))
		for i, x := range v.A {
			parts[i] = x.Repr()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	parts := make([]string, len(v.Keys))
	for i, k := range v.Keys {
		parts[i] = StrRepr(k) + ": " + v.M[k].Repr()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// Str mirrors str(): a str prints raw, anything else as its repr.
func (v Value) Str() string {
	if v.Kind == Str {
		return v.S
	}
	return v.Repr()
}
