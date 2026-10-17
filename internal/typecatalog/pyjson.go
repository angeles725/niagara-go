package typecatalog

import (
	"fmt"
	"strconv"
	"strings"
)

// The json.dumps subset this tool prints (pending: internal/pyjson, decision D26).

// kv is one member of a JSON object, in output order.
type kv struct {
	K string
	V any
}

// dumps is json.dumps(v, indent=indent) with ensure_ascii: v holds string, int64, bool, nil,
// []string, []any and []kv (an object, members in the given order).
func dumps(v any, indent int) string {
	var b strings.Builder
	encode(&b, v, indent, 0)
	return b.String()
}

func encode(b *strings.Builder, v any, indent, level int) {
	nl := func(l int) { b.WriteString("\n" + strings.Repeat(" ", indent*l)) }
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case string:
		quote(b, x)
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		encode(b, items, indent, level)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, it := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			encode(b, it, indent, level+1)
		}
		nl(level)
		b.WriteByte(']')
	case []kv:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, m := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			quote(b, m.K)
			b.WriteString(": ")
			encode(b, m.V, indent, level+1)
		}
		nl(level)
		b.WriteByte('}')
	default:
		panic(fmt.Sprintf("dumps: unsupported %T", v))
	}
}

// quote is json's ensure_ascii string encoder: every character outside ' '..'~' is escaped.
func quote(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r >= ' ' && r <= '~':
			b.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(b, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(b, `\u%04x`, r)
		}
	}
	b.WriteByte('"')
}
