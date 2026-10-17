package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// TODO(pyjson): replace with internal/pyjson once it lands on main; this is the
// subset module-nav needs (json.dumps with ensure_ascii on).

// Dumps is json.dumps(v, indent=indent) with ensure_ascii on; indent < 0 is
// indent=None (", " and ": " separators on one line). v holds decoded index
// values (nil, bool, string, json.Number, []any, *index.Object), []string, or
// Go ints and floats.
func Dumps(v any, indent int) string {
	var b strings.Builder
	dump(&b, v, indent, 0)
	return b.String()
}

func dump(b *strings.Builder, v any, indent, level int) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		quote(b, t)
	case json.Number:
		if index.IsInt(t) {
			b.WriteString(string(t))
		} else {
			f, _ := index.Float(t)
			b.WriteString(pycompat.JSONFloat(f))
		}
	case int:
		fmt.Fprint(b, t)
	case int64:
		fmt.Fprint(b, t)
	case float64:
		b.WriteString(pycompat.JSONFloat(t))
	case []string:
		items := make([]any, len(t))
		for i, s := range t {
			items[i] = s
		}
		dumpList(b, items, indent, level)
	case []any:
		dumpList(b, t, indent, level)
	case *index.Object:
		if t == nil || t.Len() == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range t.Keys() {
			separate(b, i, indent, level+1)
			quote(b, k)
			b.WriteString(": ")
			dump(b, t.Values()[i], indent, level+1)
		}
		closeWith(b, '}', indent, level)
	default:
		quote(b, fmt.Sprint(t))
	}
}

func dumpList(b *strings.Builder, items []any, indent, level int) {
	if len(items) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteByte('[')
	for i, x := range items {
		separate(b, i, indent, level+1)
		dump(b, x, indent, level+1)
	}
	closeWith(b, ']', indent, level)
}

func separate(b *strings.Builder, i, indent, level int) {
	if i > 0 {
		b.WriteByte(',')
		if indent < 0 {
			b.WriteByte(' ')
		}
	}
	if indent >= 0 {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", indent*level))
	}
}

func closeWith(b *strings.Builder, c byte, indent, level int) {
	if indent >= 0 {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", indent*level))
	}
	b.WriteByte(c)
}

// quote is json's ensure_ascii string encoder: \" \\ \n \r \t \b \f, other
// control characters and every non-ASCII code point as \uXXXX (UTF-16 pairs
// above U+FFFF).
func quote(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
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
			case r < 0x20 || (r > 0x7e && r < 0x10000):
				fmt.Fprintf(b, `\u%04x`, r)
			case r >= 0x10000:
				r -= 0x10000
				fmt.Fprintf(b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			default:
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
