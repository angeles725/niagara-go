package pyjson

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// encodeStringRef is the rune-by-rune EncodeString the fast path replaced, kept
// as the oracle the new encoder must match byte for byte.
func encodeStringRef(s string, ensureASCII bool) string {
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

// dumpsRef is the Dumps the allocation-free layout replaced (indent strings
// rebuilt per container, keys sorted by SortedKeys), over encodeStringRef.
func dumpsRef(v Value, o DumpOpts, level int) string {
	open := func(c string) (string, string, string) {
		if o.Indent < 0 {
			return c, o.ItemSep, ""
		}
		nl := "\n" + strings.Repeat(" ", o.Indent*(level+1))
		return c + nl, o.ItemSep + nl, "\n" + strings.Repeat(" ", o.Indent*level)
	}
	switch v.Kind {
	case Null:
		return "null"
	case Bool:
		return map[bool]string{true: "true", false: "false"}[v.B]
	case Int:
		return v.I.String()
	case Float:
		return pycompat.JSONFloat(v.F)
	case Str:
		return encodeStringRef(v.S, o.EnsureASCII)
	case Array:
		if len(v.A) == 0 {
			return "[]"
		}
		start, sep, end := open("[")
		parts := make([]string, len(v.A))
		for i, x := range v.A {
			parts[i] = dumpsRef(x, o, level+1)
		}
		return start + strings.Join(parts, sep) + end + "]"
	}
	if len(v.Keys) == 0 {
		return "{}"
	}
	keys := v.Keys
	if o.SortKeys {
		keys = SortedKeys(keys)
	}
	start, sep, end := open("{")
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = encodeStringRef(k, o.EnsureASCII) + o.KeySep + dumpsRef(v.M[k], o, level+1)
	}
	return start + strings.Join(parts, sep) + end + "}"
}

// wtf8Pieces are the byte sequences random strings are built from: plain and
// escaped ASCII, DEL, 2/3/4-byte UTF-8 (U+FFFD itself included), lone and paired
// surrogates as WTF-8, overlong and truncated sequences, and stray bytes.
var wtf8Pieces = []string{
	"a", "Z", "0", " ", "~", "/", "'", "\"", "\\", "\n", "\r", "\t", "\b", "\f",
	"\x00", "\x01", "\x1f", "\x7f", "é", "\u00a0", "\u0378", "中", "\uffff", "\ufffd",
	"😀", "\U0010ffff", "\xed\xa0\x80", "\xed\xbf\xbf", "\xed\xa0\xbd\xed\xb8\x80",
	"\xed\xa0", "\xed", "\xc3", "\xe4\xb8", "\xf0\x9f\x98", "\xc0\xaf", "\xe0\x80\xaf",
	"\x80", "\xbf", "\xfe", "\xff", "\xf4\x90\x80\x80",
}

func randomWTF8(rng *rand.Rand) string {
	var b strings.Builder
	for n := rng.IntN(24); n > 0; n-- {
		if rng.IntN(4) == 0 {
			b.WriteByte(byte(rng.IntN(256)))
		} else {
			b.WriteString(wtf8Pieces[rng.IntN(len(wtf8Pieces))])
		}
	}
	return b.String()
}

func TestEncodeStringMatchesReference(t *testing.T) {
	cases := append([]string{"", "plain ascii", strings.Repeat("x", 300) + "é"}, wtf8Pieces...)
	rng := rand.New(rand.NewPCG(7, 11))
	for range 20000 {
		cases = append(cases, randomWTF8(rng))
	}
	for _, s := range cases {
		for _, ensure := range []bool{false, true} {
			if got, want := EncodeString(s, ensure), encodeStringRef(s, ensure); got != want {
				t.Fatalf("EncodeString(%q, %v):\n got %q\nwant %q", s, ensure, got, want)
			}
		}
	}
}

// TestEncodeStringPythonTable pins outputs Python 3.15 json.dumps gives (and the
// WTF-8 cases the decoder produces): the escape table on both ensure_ascii sides.
func TestEncodeStringPythonTable(t *testing.T) {
	for _, c := range []struct{ in, ascii, raw string }{
		{"a\"b\\c", `"a\"b\\c"`, `"a\"b\\c"`},
		{"\n\r\t\b\f\x00\x1f", `"\n\r\t\b\f\u0000\u001f"`, `"\n\r\t\b\f\u0000\u001f"`},
		{"\x7f", `"\u007f"`, "\"\x7f\""},
		{"é中😀", `"\u00e9\u4e2d\ud83d\ude00"`, `"é中😀"`},
		{"\xed\xa0\x80x", `"\ud800x"`, "\"\xed\xa0\x80x\""},
		{"a\xffb", `"a\ufffdb"`, "\"a\ufffdb\""},
	} {
		if got := EncodeString(c.in, true); got != c.ascii {
			t.Errorf("ensure_ascii %q: got %s want %s", c.in, got, c.ascii)
		}
		if got := EncodeString(c.in, false); got != c.raw {
			t.Errorf("raw %q: got %s want %s", c.in, got, c.raw)
		}
	}
}

func FuzzEncodeString(f *testing.F) {
	for _, s := range wtf8Pieces {
		f.Add(s)
	}
	f.Add("x\xed\xa0\x80\xed\xb0\x80é\x7f\"")
	f.Fuzz(func(t *testing.T, s string) {
		for _, ensure := range []bool{false, true} {
			if got, want := EncodeString(s, ensure), encodeStringRef(s, ensure); got != want {
				t.Fatalf("EncodeString(%q, %v): got %q want %q", s, ensure, got, want)
			}
		}
	})
}

// randomValue builds a nested value whose strings and keys are random WTF-8.
func randomValue(rng *rand.Rand, depth int) Value {
	switch k := rng.IntN(9); {
	case depth > 3 || k < 4:
		return []Value{{}, BoolValue(k%2 == 0), IntValue(rng.Int64N(1000) - 500),
			FloatValue(rng.NormFloat64()), String(randomWTF8(rng))}[rng.IntN(5)]
	case k < 6:
		a := make([]Value, rng.IntN(4))
		for i := range a {
			a[i] = randomValue(rng, depth+1)
		}
		return Value{Kind: Array, A: a}
	default:
		o := NewObject()
		for n := rng.IntN(5); n > 0; n-- {
			o.Set(randomWTF8(rng), randomValue(rng, depth+1))
		}
		return o
	}
}

func TestDumpsMatchesReference(t *testing.T) {
	opts := []DumpOpts{
		Canonical, Indent2, Defaults(-1), Defaults(0), Defaults(4),
		{Indent: -1, ItemSep: ",", KeySep: ":"},
		{SortKeys: true, Indent: 2, ItemSep: ",", KeySep: ": "},
		{Indent: 70, ItemSep: ",", KeySep: ": "},
	}
	rng := rand.New(rand.NewPCG(3, 5))
	values := []Value{methodIndexLike(50)}
	for range 2000 {
		values = append(values, randomValue(rng, 0))
	}
	for _, v := range values {
		for _, o := range opts {
			if got, want := Dumps(v, o), dumpsRef(v, o, 0); got != want {
				t.Fatalf("Dumps(%+v):\n got %q\nwant %q", o, got, want)
			}
		}
	}
}
