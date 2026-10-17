package bogwrite

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Expected values below were printed by tools/bog-write.py (Python 3.15).

func TestUniqueNameMatchesObjectUtil(t *testing.T) {
	set := func(names ...string) map[string]bool {
		m := map[string]bool{}
		for _, n := range names {
			m[n] = true
		}
		return m
	}
	for _, c := range []struct {
		base     string
		existing map[string]bool
		want     string
	}{
		{"Foo", set("Foo"), "Foo1"}, {"Foo9", set("Foo9"), "Foo10"}, {"Foo", set(), "Foo"},
		{"Link", set("Link", "Link1"), "Link2"}, {"Foo009", set("Foo009"), "Foo0010"}, {"12", set("12"), "13"},
		{"Foo\u0669", set("Foo\u0669"), "Foo10"},                 // \d is any Unicode decimal digit
		{"x\u0660\u0669", set("x\u0660\u0669"), "x10"},           // only ASCII '0' padding is kept
		{"a\n", set("a\n"), "a\n1"}, {"a1\n", set("a1\n"), "a2"}, // $ matches before a final newline
	} {
		if got, err := uniqueName(c.base, c.existing); err != nil || got != c.want {
			t.Errorf("uniqueName(%q) = %q, %v; want %q", c.base, got, err, c.want)
		}
	}
}

func mustPlan(t *testing.T, text string) pyjson.Value {
	t.Helper()
	v, err := decodePlan(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPyIntOfFollowsInt(t *testing.T) {
	ws := mustPlan(t, `[" \u0661_2 ", true, 2.9, -2.9, "1__0", [1], null, 1e999]`).A
	want := []string{"12", "1", "2", "-2",
		"ValueError: invalid literal for int() with base 10: '1__0'",
		"TypeError: int() argument must be a string, a bytes-like object or a real number, not 'list'",
		"TypeError: int() argument must be a string, a bytes-like object or a real number, not 'NoneType'",
		"OverflowError: cannot convert float infinity to integer"}
	for i, v := range ws {
		got, err := pyIntOf(v)
		if err != nil {
			got = err.Error()
		}
		if got != want[i] {
			t.Errorf("int(%s) = %q, want %q", v.Repr(), got, want[i])
		}
	}
}

func TestIterateYieldsCharactersOfAStr(t *testing.T) {
	items, err := iterate(mustPlan(t, `"a\u00e9\ud800\ud83d\ude00"`))
	if err != nil {
		t.Fatal(err)
	}
	var reprs []string
	for _, it := range items {
		reprs = append(reprs, it.Repr())
	}
	if got := strings.Join(reprs, " "); got != `'a' 'é' '\ud800' '😀'` {
		t.Errorf("iter(str) = %s", got)
	}
	if _, err := iterate(mustPlan(t, `5`)); err == nil || err.Error() != "TypeError: 'int' object is not iterable" {
		t.Errorf("iter(5): %v", err)
	}
}

// TestDecodePlanNamesTheException: the traceback's last line names the exception. The
// RecursionError text is pyjson's fixed-depth one (CPython 3.15 reports its C stack use).
func TestDecodePlanNamesTheException(t *testing.T) {
	for text, want := range map[string]string{
		"{":                        "json.decoder.JSONDecodeError: Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
		strings.Repeat("9", 4301):  "ValueError: Exceeds the limit (4300 digits) for integer string conversion: value has 4301 digits; use sys.set_int_max_str_digits() to increase the limit",
		strings.Repeat("[", 60000): "RecursionError: maximum recursion depth exceeded while decoding a JSON array from a unicode string",
	} {
		if _, err := decodePlan(text); err == nil || err.Error() != want {
			t.Errorf("decodePlan(%.20q...): %v, want %q", text, err, want)
		}
	}
}

func TestValidateXMLFindings(t *testing.T) {
	base := func(body string) []byte {
		return []byte("<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='R' h='1' m='b=baja' t='b:Folder'>" +
			body + "</p></bajaObjectGraph>")
	}
	ws := func(v string) []byte {
		return base("<p n='A' h='2' t='b:Folder'><p n='wsAnnotation' t='b:WsAnnotation' v='" + v + "'/></p>")
	}
	for _, c := range []struct {
		name string
		doc  []byte
		want string // findings joined by "; ", "" for none
	}{
		{"clean", base("<p n='A' h='2' t='b:Folder'/>"), ""},
		{"not well-formed", []byte("<bajaObjectGraph><p></bajaObjectGraph>"),
			"('ERROR', 'XML not well-formed: mismatched tag: line 1, column 22')"},
		{"wrong root", []byte("<x/>"), `('ERROR', "root element is 'x', expected bajaObjectGraph")`},
		{"m= split on white space", base("<p n='A' h='2' m=' kit = kitControl ' t='kit:Thing'/>"),
			`('ERROR', "type prefix 'kit' used before declared at R/A")`},
		{"bad slot name", base("<p n='9a' h='2' t='b:Folder'/>"), `('ERROR', "illegal slot name '9a' at R/9a")`},
		{"ws Unicode digit", ws("\u0661,-2,3|x"), ""},
		{"ws final newline", ws("1,2,3,4&#10;"), ""},
		{"ws two newlines", ws("1,2,3&#10;&#10;"),
			`('ERROR', "bad wsAnnotation value '1,2,3\\n\\n' at R/A/wsAnnotation (expected x,y,w[,h])")`},
	} {
		var got []string
		for _, f := range validateXML(c.doc) {
			got = append(got, f.repr())
		}
		if g := strings.Join(got, "; "); g != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, g, c.want)
		}
	}
}
