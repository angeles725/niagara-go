package bognav

import (
	"bytes"
	"io"
	"testing"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// TestRenderCSV pins render's --csv path end to end through pycsv: the header from cols,
// csv.writer's "\r\n" terminator and minimal quoting, and None (nil or a nil *string) as an
// empty field. pycsv's own quoting is checked against Python's fixtures in internal/pycsv.
func TestRenderCSV(t *testing.T) {
	ns := argparse.NewNamespace()
	for k, v := range map[string]any{"json": false, "csv": true, "limit": 0} {
		ns.Set(k, v)
	}
	var out bytes.Buffer
	e := &env{ns: ns, out: &out, err: io.Discard}
	var none *string
	rows := []row{
		{{"name", "a,b"}, {"value", `say "hi"`}, {"note", nil}},
		{{"name", "plain"}, {"value", none}, {"note", true}},
	}
	if n := e.render(rows, nil, "no rows", []string{"name", "value", "note"}); n != 2 {
		t.Errorf("render returned %d rows, want 2", n)
	}
	want := "name,value,note\r\n\"a,b\",\"say \"\"hi\"\"\",\r\nplain,,True\r\n"
	if got := out.String(); got != want {
		t.Errorf("--csv output %q\nwant         %q", got, want)
	}
}

func TestJSONEscaping(t *testing.T) {
	got, _ := dumps(row{{"s", "é\"\\\n\x7f😀"}, {"n", nil}, {"b", true}, {"l", []any{}}, {"o", row{}}}, -1)
	if want := `{"s": "\u00e9\"\\\n\u007f\ud83d\ude00", "n": null, "b": true, "l": [], "o": {}}`; got != want {
		t.Errorf("dumps %q\nwant  %q", got, want)
	}
}

func TestPyRepr(t *testing.T) {
	cases := map[string]string{"x": "'x'", "it's": `"it's"`, `a'b"c`: `'a\'b"c'`, "tab\t\x01é ": `'tab\t\x01é\xa0'`}
	for in, want := range cases {
		if got := pycompat.StrRepr(in); got != want {
			t.Errorf("StrRepr(%q) = %s, want %s", in, got, want)
		}
	}
}

// An unsupported value is an error, never a panic.
func TestDumpsRejectsUnsupportedTypes(t *testing.T) {
	if _, err := dumps(row{{"x", []int{1}}}, 2); err == nil || err.Error() != "pyjson.FromAny: unsupported []int" {
		t.Errorf("err = %v", err)
	}
	if got, err := dumps([]any{"a", 1}, -1); err != nil || got != `["a", 1]` {
		t.Errorf("got %q err %v", got, err)
	}
}

// Parity with classify.py writability: only a simple type name starting with "Status" is
// complex; a B-class spelling (BStatusNumeric) ends in Numeric and is simple, as in Python.
func TestWritabilityStatusParity(t *testing.T) {
	for typ, want := range map[string]string{"b:StatusNumeric": "complex", "StatusBoolean": "complex",
		"BStatusNumeric": "simple", "b:BStatusEnum": "simple", "x:BStatusFoo": "other"} {
		if got, _ := writability(typ); got != want {
			t.Errorf("%s -> %s, want %s", typ, got, want)
		}
	}
}
