package typecatalog

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// The json.dumps subset this tool prints; local until internal/pyjson lands (decision D26).

// kv is one member of a JSON object, in output order.
type kv struct {
	K string
	V any
}

// dumps is json.dumps(v, indent=indent) with ensure_ascii: v holds string, int64, *big.Int,
// bool, nil, []string, []any and []kv (an object, members in the given order). Any other type
// is an error.
func dumps(v any, indent int) (string, error) {
	var b strings.Builder
	if err := encode(&b, v, indent, 0); err != nil {
		return "", err
	}
	return b.String(), nil
}

func encode(b *strings.Builder, v any, indent, level int) error {
	nl := func(l int) { b.WriteString("\n" + strings.Repeat(" ", indent*l)) }
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case *big.Int:
		if x == nil {
			return fmt.Errorf("json: nil *big.Int")
		}
		b.WriteString(x.String())
	case string:
		quote(b, x)
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		return encode(b, items, indent, level)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteByte('[')
		for i, it := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			if err := encode(b, it, indent, level+1); err != nil {
				return err
			}
		}
		nl(level)
		b.WriteByte(']')
	case []kv:
		if len(x) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteByte('{')
		for i, m := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			quote(b, m.K)
			b.WriteString(": ")
			if err := encode(b, m.V, indent, level+1); err != nil {
				return err
			}
		}
		nl(level)
		b.WriteByte('}')
	default:
		return fmt.Errorf("json: unsupported value of type %T", v)
	}
	return nil
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
