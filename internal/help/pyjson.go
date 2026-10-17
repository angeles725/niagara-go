package help

// json.dumps(v, indent=2) with the default ensure_ascii=True, for the decoded
// index values and the ordered objects the --json views build.
//
// TODO(pyjson): replace with the shared internal/pyjson encoder once it is on
// main (plan decision D26); bognav and typecatalog carry the same private copy.

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// jsonKV is one key of a jsonObj.
type jsonKV struct {
	K string
	V any
}

// jsonObj is a dict literal built by a command: its keys print in order.
type jsonObj []jsonKV

// dumpsIndent2 is json.dumps(v, indent=2).
func dumpsIndent2(v any) string {
	var b strings.Builder
	encodeJSON(&b, v, 0)
	return b.String()
}

func encodeJSON(b *strings.Builder, v any, level int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		quoteJSON(b, x)
	case json.Number:
		b.WriteString(jsonNumber(x))
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		encodeJSON(b, items, level)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, it := range x {
			jsonSep(b, i, level+1)
			encodeJSON(b, it, level+1)
		}
		jsonClose(b, level, ']')
	case *index.Object:
		kvs := make(jsonObj, 0, x.Len())
		for _, k := range x.Keys() {
			val, _ := x.Get(k)
			kvs = append(kvs, jsonKV{k, val})
		}
		encodeJSON(b, kvs, level)
	case jsonObj:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, kv := range x {
			jsonSep(b, i, level+1)
			quoteJSON(b, kv.K)
			b.WriteString(": ")
			encodeJSON(b, kv.V, level+1)
		}
		jsonClose(b, level, '}')
	default:
		panic(fmt.Sprintf("dumpsIndent2: unsupported %T", v))
	}
}

// jsonSep starts item i of a container at nesting level: "," after the
// previous item, then a new line indented by two spaces per level.
func jsonSep(b *strings.Builder, i, level int) {
	if i > 0 {
		b.WriteByte(',')
	}
	b.WriteString("\n" + strings.Repeat("  ", level))
}

func jsonClose(b *strings.Builder, level int, c byte) {
	b.WriteString("\n" + strings.Repeat("  ", level))
	b.WriteByte(c)
}

// jsonNumber re-prints a decoded JSON number the way Python does after
// json.load: a literal with a fraction or exponent became a float (printed
// with repr), any other literal an int of any size ("-0" prints as "0").
func jsonNumber(n json.Number) string {
	lit := n.String()
	if strings.ContainsAny(lit, ".eE") {
		f, _ := strconv.ParseFloat(lit, 64) // out of range: ±Inf, as float() gives
		return pycompat.JSONFloat(f)
	}
	i, ok := new(big.Int).SetString(lit, 10)
	if !ok {
		return lit
	}
	return i.String()
}

// quoteJSON is the ensure_ascii string encoder: '"', '\\' and the short
// control escapes, printable ASCII as is, everything else as \uXXXX (UTF-16
// surrogate pairs above U+FFFF).
func quoteJSON(b *strings.Builder, s string) {
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
