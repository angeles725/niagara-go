package pyjson

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// EncodeString mirrors json's py_encode_basestring(_ascii).
func EncodeString(s string, ensureASCII bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range Runes(s) {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(&b, `\u%04x`, r)
			case r < 0x7F || !ensureASCII:
				b.WriteString(WTF8([]rune{r}))
			case r > 0xFFFF:
				r -= 0x10000
				fmt.Fprintf(&b, `\u%04x\u%04x`, 0xD800|(r>>10)&0x3FF, 0xDC00|r&0x3FF)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

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
	open := func(c byte) (string, string) {
		if o.Indent < 0 {
			return string(c), o.ItemSep
		}
		nl := "\n" + strings.Repeat(" ", o.Indent*(level+1))
		return string(c) + nl, o.ItemSep + nl
	}
	closing := func(c byte) {
		if o.Indent >= 0 {
			b.WriteString("\n" + strings.Repeat(" ", o.Indent*level))
		}
		b.WriteByte(c)
	}
	switch v.Kind {
	case Null:
		b.WriteString("null")
	case Bool:
		b.WriteString(map[bool]string{true: "true", false: "false"}[v.B])
	case Int:
		b.WriteString(v.I.String())
	case Float:
		b.WriteString(pycompat.JSONFloat(v.F))
	case Str:
		b.WriteString(EncodeString(v.S, o.EnsureASCII))
	case Array:
		if len(v.A) == 0 {
			b.WriteString("[]")
			return
		}
		start, sep := open('[')
		b.WriteString(start)
		for i, x := range v.A {
			if i > 0 {
				b.WriteString(sep)
			}
			dumpTo(b, x, o, level+1)
		}
		closing(']')
	case Object:
		if len(v.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		keys := v.Keys
		if o.SortKeys {
			keys = SortedKeys(keys)
		}
		start, sep := open('{')
		b.WriteString(start)
		for i, k := range keys {
			if i > 0 {
				b.WriteString(sep)
			}
			b.WriteString(EncodeString(k, o.EnsureASCII))
			b.WriteString(o.KeySep)
			dumpTo(b, v.M[k], o, level+1)
		}
		closing('}')
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
