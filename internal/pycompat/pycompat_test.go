package pycompat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The fixtures and digests are written by testdata/gen.py from Python 3.15.

type fixtures struct {
	Python   string              `json:"python"`
	Unicode  string              `json:"unicode"`
	Repr     [][]json.RawMessage `json:"repr"`
	Float    [][]json.RawMessage `json:"float"`
	ParseInt [][]json.RawMessage `json:"parse_int"`
	ParseFlt [][]json.RawMessage `json:"parse_float"`
	Percent  [][]json.RawMessage `json:"percent"`
	Decode   [][]json.RawMessage `json:"decode"`
	Lines    [][]json.RawMessage `json:"lines"`
	Spaces   [][]json.RawMessage `json:"spaces"`
	HTML     [][]json.RawMessage `json:"html"`
	Paths    [][]json.RawMessage `json:"paths"`
	Joins    [][]json.RawMessage `json:"joins"`
	Digests  map[string]string   `json:"digests"`
}

func load(t *testing.T) fixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
}

func fromBits(t *testing.T, h string) float64 {
	t.Helper()
	u, err := strconv.ParseUint(h, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

// typed decodes gen.py's [kind, value] pairs into the Go values Repr accepts.
func typed(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var pair []json.RawMessage
	decode(t, raw, &pair)
	var kind string
	decode(t, pair[0], &kind)
	switch kind {
	case "none":
		return nil
	case "bool":
		var b bool
		decode(t, pair[1], &b)
		return b
	case "int":
		var s string
		decode(t, pair[1], &s)
		n, _ := new(big.Int).SetString(s, 10)
		if n.IsInt64() {
			return int(n.Int64())
		}
		return n
	case "float":
		var s string
		decode(t, pair[1], &s)
		return fromBits(t, s)
	case "str":
		var s string
		decode(t, pair[1], &s)
		return s
	case "list", "tuple":
		var items []json.RawMessage
		decode(t, pair[1], &items)
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = typed(t, it)
		}
		if kind == "tuple" {
			return Tuple(out)
		}
		return out
	}
	t.Fatalf("unknown kind %s", kind)
	return nil
}

func TestHeaderNamesFixtureVersions(t *testing.T) {
	f := load(t)
	if !strings.Contains(UnicodeHeader(), "Python "+f.Python+", Unicode "+f.Unicode) {
		t.Errorf("unicode.txt %q is not from Python %s / Unicode %s", UnicodeHeader(), f.Python, f.Unicode)
	}
}

func TestReprAndStr(t *testing.T) {
	for _, c := range load(t).Repr {
		v := typed(t, c[0])
		var repr, str string
		decode(t, c[1], &repr)
		decode(t, c[2], &str)
		if got := Repr(v); got != repr {
			t.Errorf("Repr(%s) = %s, want %s", c[0], got, repr)
		}
		if got := Str(v); got != str {
			t.Errorf("Str(%s) = %s, want %s", c[0], got, str)
		}
	}
}

func TestFloatFormats(t *testing.T) {
	for _, c := range load(t).Float {
		var h, repr, g, js string
		var asInt *string
		decode(t, c[0], &h)
		decode(t, c[1], &repr)
		decode(t, c[2], &g)
		decode(t, c[3], &js)
		decode(t, c[4], &asInt)
		f := fromBits(t, h)
		if got := FloatRepr(f); got != repr {
			t.Errorf("FloatRepr(%s) = %s, want %s", h, got, repr)
		}
		if got := FormatG(f); got != g {
			t.Errorf("FormatG(%s) = %s, want %s", h, got, g)
		}
		if got := JSONFloat(f); got != js {
			t.Errorf("JSONFloat(%s) = %s, want %s", h, got, js)
		}
		got, ok := IntOfFloat(f)
		if ok != (asInt != nil) || ok && got != *asInt {
			t.Errorf("IntOfFloat(%s) = %s, %v; want %v", h, got, ok, asInt)
		}
	}
}

func TestParseIntAndFloat(t *testing.T) {
	f := load(t)
	for _, c := range f.ParseInt {
		var in string
		var want *string
		decode(t, c[0], &in)
		decode(t, c[1], &want)
		n, err := ParseBigInt(in)
		if (err == nil) != (want != nil) || err == nil && n.String() != *want {
			t.Errorf("ParseBigInt(%q) = %v, %v; want %v", in, n, err, want)
		}
		i, err := ParseInt(in)
		switch {
		case want == nil && err != ErrSyntax:
			t.Errorf("ParseInt(%q) error %v, want ErrSyntax", in, err)
		case want != nil && err == nil && strconv.FormatInt(i, 10) != *want:
			t.Errorf("ParseInt(%q) = %d, want %s", in, i, *want)
		case want != nil && err != nil && (err != ErrRange || n.IsInt64()):
			t.Errorf("ParseInt(%q) error %v", in, err)
		}
	}
	for _, c := range f.ParseFlt {
		var in string
		var want *string
		decode(t, c[0], &in)
		decode(t, c[1], &want)
		got, err := ParseFloat(in)
		switch {
		case want == nil:
			if err == nil {
				t.Errorf("ParseFloat(%q) = %v, want an error", in, got)
			}
		case err != nil:
			t.Errorf("ParseFloat(%q): %v", in, err)
		case math.IsNaN(fromBits(t, *want)):
			if !math.IsNaN(got) {
				t.Errorf("ParseFloat(%q) = %v, want nan", in, got)
			}
		case math.Float64bits(got) != math.Float64bits(fromBits(t, *want)):
			t.Errorf("ParseFloat(%q) = %v, want %v", in, got, fromBits(t, *want))
		}
	}
}

func TestPercent(t *testing.T) {
	for _, c := range load(t).Percent {
		var format string
		var raw map[string]json.RawMessage
		var want *string
		decode(t, c[0], &format)
		decode(t, c[1], &raw)
		decode(t, c[2], &want)
		params := map[string]any{}
		for k, v := range raw {
			params[k] = typed(t, v)
		}
		got, err := Percent(format, params)
		if (err == nil) != (want != nil) || err == nil && got != *want {
			t.Errorf("Percent(%q) = %q, %v; want %v", format, got, err, want)
		}
	}
}

func TestTextFunctions(t *testing.T) {
	f := load(t)
	for _, c := range f.Decode {
		var h, want string
		decode(t, c[0], &h)
		decode(t, c[1], &want)
		b, _ := hex.DecodeString(h)
		if got := DecodeReplace(b); got != want {
			t.Errorf("DecodeReplace(%s) = %q, want %q", h, got, want)
		}
	}
	for _, c := range f.Lines {
		var s string
		var plain, kept []string
		decode(t, c[0], &s)
		decode(t, c[1], &plain)
		decode(t, c[2], &kept)
		if got := SplitLines(s, false); strings.Join(got, "|") != strings.Join(plain, "|") || len(got) != len(plain) {
			t.Errorf("SplitLines(%q) = %q, want %q", s, got, plain)
		}
		if got := SplitLines(s, true); strings.Join(got, "|") != strings.Join(kept, "|") || len(got) != len(kept) {
			t.Errorf("SplitLines(%q, keepends) = %q, want %q", s, got, kept)
		}
	}
	for _, c := range f.Spaces {
		var s, stripped string
		var fields []string
		decode(t, c[0], &s)
		decode(t, c[1], &stripped)
		decode(t, c[2], &fields)
		if got := Strip(s); got != stripped {
			t.Errorf("Strip(%q) = %q, want %q", s, got, stripped)
		}
		if got := Fields(s); strings.Join(got, "|") != strings.Join(fields, "|") || len(got) != len(fields) {
			t.Errorf("Fields(%q) = %q, want %q", s, got, fields)
		}
	}
	for _, c := range f.HTML {
		var s, esc, unesc string
		decode(t, c[0], &s)
		decode(t, c[1], &esc)
		decode(t, c[2], &unesc)
		if got := HTMLEscape(s); got != esc {
			t.Errorf("HTMLEscape(%q) = %q, want %q", s, got, esc)
		}
		if got := HTMLUnescape(s); got != unesc {
			t.Errorf("HTMLUnescape(%q) = %q, want %q", s, got, unesc)
		}
	}
	for _, c := range f.Paths {
		var p, base, root string
		decode(t, c[0], &p)
		decode(t, c[1], &base)
		decode(t, c[2], &root)
		gotRoot, ext := PosixSplitext(p)
		if PosixBasename(p) != base || gotRoot != root || gotRoot+ext != p {
			t.Errorf("path %q: basename %q root %q ext %q; want %q %q", p, PosixBasename(p), gotRoot, ext, base, root)
		}
	}
	for _, c := range f.Joins {
		var parts []string
		var want string
		decode(t, c[0], &parts)
		decode(t, c[1], &want)
		if got := PosixJoin(parts[0], parts[1:]...); got != want {
			t.Errorf("PosixJoin(%q) = %q, want %q", parts, got, want)
		}
	}
}

func TestLineBreakLenCountsCRLFOnce(t *testing.T) {
	for s, want := range map[string]int{"\r\n": 2, "\r": 1, "\n": 1, "\u0085": 2, "\u2028": 3, "a": 0, "\r\r\n": 1} {
		if got := LineBreakLen(s, 0); got != want {
			t.Errorf("LineBreakLen(%q) = %d, want %d", s, got, want)
		}
	}
}

// digest hashes each code point accepted by pred, like gen.py digest(chars()).
func digest(each func(r rune) (string, bool)) string {
	h := sha256.New()
	for r := rune(0); r < 0x110000; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		if s, ok := each(r); ok {
			h.Write([]byte(s))
			h.Write([]byte{'\n'})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func is(pred func(rune) bool) func(rune) (string, bool) {
	return func(r rune) (string, bool) { return string(r), pred(r) }
}

func TestEveryCodePoint(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive code-point digests")
	}
	f := load(t)
	checks := map[string]func(rune) (string, bool){
		"alpha": is(IsAlpha), "decimal": is(IsDecimal), "digit": is(IsDigit), "numeric": is(IsNumeric),
		"alnum": is(IsAlnum), "printable": is(IsPrintable), "space": is(IsSpace), "word": is(IsWordRune),
		"linebreak": is(IsLineBreak),
		"repr":      func(r rune) (string, bool) { return StrRepr(string(r)), true },
		"decimal_value": func(r rune) (string, bool) {
			return string(r) + strconv.Itoa(decimalValue(r)), decimalValue(r) >= 0
		},
	}
	for name, each := range checks {
		want, ok := f.Digests[name]
		if !ok {
			t.Errorf("no %s digest in fixtures", name)
		}
		if got := digest(each); got != want {
			t.Errorf("%s: digest over every code point differs from Python", name)
		}
	}
}
