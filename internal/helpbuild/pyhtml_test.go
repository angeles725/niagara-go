package helpbuild

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Tokenizer cases whose expected events were taken from CPython 3.15's
// html.parser (htmldriver.py pages); TestParityHTMLCorners and
// TestParityHTMLFuzz re-check them against the live interpreter.
func TestParseHTMLEvents(t *testing.T) {
	for _, c := range []struct{ page, want string }{
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
	} {
		got := pyjson.Dumps(goEvents(c.page), DumpDefault)
		want := pyjson.Dumps(mustLoads(t, c.want), DumpDefault)
		if got != want {
			t.Errorf("events of %q:\n got %s\nwant %s", c.page, got, want)
		}
	}
}

// eventRecorder is htmldriver.py's _Recorder.
type eventRecorder struct{ events []pyjson.Value }

func attrsValue(attrs []htmlAttr) pyjson.Value {
	out := array()
	for _, a := range attrs {
		v := str(a.Value)
		if a.Null {
			v = pyjson.Value{}
		}
		out.A = append(out.A, array(str(a.Name), v))
	}
	return out
}

func (r *eventRecorder) startTag(tag string, attrs []htmlAttr) {
	r.events = append(r.events, array(str("s"), str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) startEndTag(tag string, attrs []htmlAttr) {
	r.events = append(r.events, array(str("se"), str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) endTag(tag string) { r.events = append(r.events, array(str("e"), str(tag))) }
func (r *eventRecorder) data(text string)  { r.events = append(r.events, array(str("d"), str(text))) }

func goEvents(page string) pyjson.Value {
	r := &eventRecorder{}
	parseHTML(page, r)
	return array(r.events...)
}

func mustLoads(t *testing.T, s string) pyjson.Value {
	t.Helper()
	v, err := pyjson.Loads(pyjson.Runes(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestIsHTML5Entity(t *testing.T) {
	for name, want := range map[string]bool{
		"amp": true, "amp;": true, "notin;": true, "notin": false, "not": true, "notit;": false,
		"NotEqualTilde;": true, "nGt;": true, "nLt;": true, "nGt": false, "alpha": false, "x": false,
	} {
		if got := isHTML5Entity(name); got != want {
			t.Errorf("isHTML5Entity(%q) = %v", name, got)
		}
	}
}
