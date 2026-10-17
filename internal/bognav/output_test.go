package bognav

import (
	"bytes"
	"testing"
)

func TestCSVQuotingAndJSONEscaping(t *testing.T) {
	var b bytes.Buffer
	csvRow(&b, []string{"a,b", `q"x`, "line\nbreak", "", "plain"})
	if want := "\"a,b\",\"q\"\"x\",\"line\nbreak\",,plain\r\n"; b.String() != want {
		t.Errorf("csv %q", b.String())
	}
	b.Reset()
	csvRow(&b, []string{""})
	if b.String() != "\"\"\r\n" {
		t.Errorf("single empty field %q", b.String())
	}
	got := dumps(row{{"s", "é\"\\\n\x7f😀"}, {"n", nil}, {"b", true}, {"l", []any{}}, {"o", row{}}}, -1)
	if want := `{"s": "\u00e9\"\\\n\u007f\ud83d\ude00", "n": null, "b": true, "l": [], "o": {}}`; got != want {
		t.Errorf("dumps %q\nwant  %q", got, want)
	}
}

func TestPyRepr(t *testing.T) {
	cases := map[string]string{"x": "'x'", "it's": `"it's"`, `a'b"c`: `'a\'b"c'`, "tab\t\x01é ": `'tab\t\x01é\xa0'`}
	for in, want := range cases {
		if got := pyRepr(in); got != want {
			t.Errorf("pyRepr(%q) = %s, want %s", in, got, want)
		}
	}
}
