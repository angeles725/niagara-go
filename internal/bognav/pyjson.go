package bognav

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
)

// The json.dumps subset bog-nav prints; local until internal/pyjson lands (decision D26).

// dumps is json.dumps(v, indent=indent) with ensure_ascii; indent < 0 is indent=None (one line,
// ", " and ": " separators).
// Any other value type is an error, never a panic.
func dumps(v any, indent int) (string, error) {
	var b strings.Builder
	if err := encode(&b, v, indent, 0); err != nil {
		return "", err
	}
	return b.String(), nil
}

// printJSON prints dumps(v, indent) and a newline; on an encoder error it prints one stderr
// line instead and marks the run failed (Main exits 2).
func (e *env) printJSON(v any, indent int) {
	s, err := dumps(v, indent)
	if err != nil {
		fmt.Fprintf(e.err, "bog-nav: %v\n", err)
		e.failed = true
		return
	}
	e.println(s)
}

func encode(b *strings.Builder, v any, indent, level int) error {
	sep := func(first bool, l int) {
		if !first {
			b.WriteByte(',')
			if indent < 0 {
				b.WriteByte(' ')
			}
		}
		if indent >= 0 {
			b.WriteString("\n" + strings.Repeat(" ", indent*l))
		}
	}
	end := func() {
		if indent >= 0 {
			b.WriteString("\n" + strings.Repeat(" ", indent*level))
		}
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		quote(b, x)
	case *string:
		if x == nil {
			b.WriteString("null")
		} else {
			quote(b, *x)
		}
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		return encode(b, items, indent, level)
	case []row:
		items := make([]any, len(x))
		for i, r := range x {
			items[i] = r
		}
		return encode(b, items, indent, level)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteByte('[')
		for i, it := range x {
			sep(i == 0, level+1)
			if err := encode(b, it, indent, level+1); err != nil {
				return err
			}
		}
		end()
		b.WriteByte(']')
	case row:
		if len(x) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteByte('{')
		for i, m := range x {
			sep(i == 0, level+1)
			quote(b, m.K)
			b.WriteString(": ")
			if err := encode(b, m.V, indent, level+1); err != nil {
				return err
			}
		}
		end()
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

// slotJSON is a stored slot as the Python dict {type, value, flags[, child]}.
func slotJSON(s *bog.Slot) row {
	r := row{{"type", s.Type}, {"value", s.Value}, {"flags", s.Flags}}
	if s.Child.Len() > 0 {
		var child row
		for k, v := range s.Child.All() {
			child = append(child, kv{k, v})
		}
		r = append(r, kv{"child", child})
	}
	return r
}

// orderedStrings is a bog.Ordered[string] as a JSON object.
func orderedStrings(o *bog.Ordered[string]) row {
	r := row{}
	for k, v := range o.All() {
		r = append(r, kv{k, v})
	}
	return r
}
