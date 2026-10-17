package corpusnav

import (
	"fmt"
	"sort"
	"strings"
)

// kv / obj are an ordered JSON object (a Python dict keeps insertion order).
type kv struct {
	key string
	val any
}

type obj []kv

// jsonDumps is json.dumps(v, ensure_ascii=False) without indent: ", " and ": ".
func jsonDumps(v any) string {
	switch x := v.(type) {
	case string:
		return jsonString(x)
	case int:
		return fmt.Sprint(x)
	case []any:
		parts := make([]string, len(x))
		for i, it := range x {
			parts[i] = jsonDumps(it)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case obj:
		parts := make([]string, len(x))
		for i, f := range x {
			parts[i] = jsonString(f.key) + ": " + jsonDumps(f.val)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	panic(fmt.Sprintf("jsonDumps: unsupported %T", v))
}

// jsonString is a JSON string with ensure_ascii=False: only ", \ and controls escaped.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
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
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func sortStable(xs []string, less func(a, b string) bool) {
	sort.SliceStable(xs, func(i, j int) bool { return less(xs[i], xs[j]) })
}
