package pyhtml

import (
	"encoding/json"
	"testing"
)

// The events are compared as JSON values through encoding/json: pyjson
// imports pycompat, which imports this package.

// eventCases are tokenizer cases whose expected events were taken from
// CPython 3.15's html.parser (helpbuild's htmldriver.py pages);
// helpbuild's TestParityHTMLCorners and TestParityHTMLFuzz re-check the
// parser against the live interpreter.
var eventCases = []struct{ page, want string }{
	{"<p class=x>a &amp; b</p>", `[["s", "p", [["class", "x"]]], ["d", "a & b"], ["e", "p"]]`},
	{"<A HREF='X'>", `[["s", "a", [["href", "X"]]]]`},
	{"<img alt>", `[["s", "img", [["alt", null]]]]`},
	{"<img alt=>", `[["s", "img", [["alt", ""]]]]`},
	{"<br/>", `[["se", "br", []]]`},
	{"<a x='1'/>", `[["se", "a", [["x", "1"]]]]`},
	{"<a x= 'y>", `[["s", "a", [["x", ""], ["'y", null]]]]`},
	{"<a x='y>", `[]`}, // never closed: the rest of the page is dropped
	{`<img alt="&amp;&ampy&lt=&#65;&notin;&notit;">`, `[["s", "img", [["alt", "&&ampy&lt=A\u2209&notit;"]]]]`},
	{"<script>a<b>&amp;</script >c", `[["s", "script", []], ["d", "a<b>&amp;"], ["e", "script"], ["d", "c"]]`},
	{"<title>&lt;b&gt;</title>", `[["s", "title", []], ["d", "<b>"], ["e", "title"]]`},
	{"<plaintext></plaintext>&amp;", `[["s", "plaintext", []], ["d", "</plaintext>&amp;"]]`},
	{"a <!-- c --> b <!doctype x> <?pi?> <![CDATA[z]]> <!x>", `[["d", "a "], ["d", " b "], ["d", " "], ["d", " "], ["d", " "]]`},
	{"x </> y </1 z> w", `[["d", "x "], ["d", " y "], ["d", " w"]]`},
	{"a < b", `[["d", "a "], ["d", "<"], ["d", " b"]]`},
	{"x </", `[["d", "x "], ["d", "</"]]`},
	{"<\u212a>", `[["d", "<"], ["d", "\u212a>"]]`},
	{"<a\x0b>", `[["s", "a\u000b", []]]`},
	{"<x =y>", `[["s", "x", [["=y", null]]]]`},
	{"&#xD800;&#128;&nGt;", `[["d", "\ufffd\u20ac\u226b\u20d2"]]`},
	// Raw text left open at the end of the page: the rest is data.
	{"<script>a<b", `[["s", "script", []], ["d", "a<b"]]`},
	{"<style>p{} </sty", `[["s", "style", []], ["d", "p{} </sty"]]`},
	{"<xmp>x</xmp", `[["s", "xmp", []], ["d", "x</xmp"]]`},
	// The raw-text end tag matches ASCII case-insensitively, before any delimiter.
	{"<SCRIPT>x</SCRIPT>y", `[["s", "script", []], ["d", "x"], ["e", "script"], ["d", "y"]]`},
	{"<STYLE>a</STYLE >b", `[["s", "style", []], ["d", "a"], ["e", "style"], ["d", "b"]]`},
	{"<script>x</SCRIPT\n>y", `[["s", "script", []], ["d", "x"], ["e", "script"], ["d", "y"]]`},
	// textarea is RCDATA: charrefs unescaped, tags kept as text.
	{"<textarea>&lt;b&gt; <i>x</i></textarea>z", `[["s", "textarea", []], ["d", "<b> <i>x</i>"], ["e", "textarea"], ["d", "z"]]`},
	{"<TEXTAREA>a&amp;b</TEXTAREA>c", `[["s", "textarea", []], ["d", "a&b"], ["e", "textarea"], ["d", "c"]]`},
	{"<textarea>never &amp; closed", `[["s", "textarea", []], ["d", "never & closed"]]`},
	{"<textarea>a</textareax>b</textarea>c", `[["s", "textarea", []], ["d", "a</textareax>b"], ["e", "textarea"], ["d", "c"]]`},
	{"<title>a</TITLE>b", `[["s", "title", []], ["d", "a"], ["e", "title"], ["d", "b"]]`},
}

func TestParseEvents(t *testing.T) {
	for _, c := range eventCases {
		got := canonical(t, events(c.page))
		var want any
		if err := json.Unmarshal([]byte(c.want), &want); err != nil {
			t.Fatalf("%s: %v", c.want, err)
		}
		if w := canonical(t, want); got != w {
			t.Errorf("events of %q:\n got %s\nwant %s", c.page, got, w)
		}
	}
}

func canonical(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// eventRecorder is helpbuild htmldriver.py's _Recorder: ["s", tag, attrs],
// ["se", tag, attrs], ["e", tag], ["d", data], an attribute [name, value]
// with nil for Python's None.
type eventRecorder struct{ events []any }

func attrsValue(attrs []Attr) []any {
	out := []any{}
	for _, a := range attrs {
		var v any = a.Value
		if a.Null {
			v = nil
		}
		out = append(out, []any{a.Name, v})
	}
	return out
}

func (r *eventRecorder) StartTag(tag string, attrs []Attr) {
	r.events = append(r.events, []any{"s", tag, attrsValue(attrs)})
}
func (r *eventRecorder) StartEndTag(tag string, attrs []Attr) {
	r.events = append(r.events, []any{"se", tag, attrsValue(attrs)})
}
func (r *eventRecorder) EndTag(tag string) { r.events = append(r.events, []any{"e", tag}) }
func (r *eventRecorder) Data(text string)  { r.events = append(r.events, []any{"d", text}) }

func events(page string) []any {
	r := &eventRecorder{events: []any{}}
	Parse(page, r)
	return r.events
}
