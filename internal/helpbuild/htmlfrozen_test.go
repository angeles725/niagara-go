package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The S5 tokenizer and converter (pyhtml.Parse, pyhtml.Feed, HTMLToText)
// and the html5 entity table against their frozen Python output
// (frozen_test.go): html.parser's callbacks and html_text.convert on the
// corner pages, one feed() without close() on them and on the feed corners,
// and html.entities.html5 with html.unescape around every name.

// eventRecorder records the pyhtml callbacks as html.parser's would be.
type eventRecorder struct{ events []pyjson.Value }

func attrsValue(attrs []pyhtml.Attr) pyjson.Value {
	out := testkit.Array()
	for _, a := range attrs {
		v := testkit.Str(a.Value)
		if a.Null {
			v = pyjson.Value{}
		}
		out.A = append(out.A, testkit.Array(testkit.Str(a.Name), v))
	}
	return out
}

func (r *eventRecorder) StartTag(tag string, attrs []pyhtml.Attr) {
	r.events = append(r.events, testkit.Array(testkit.Str("s"), testkit.Str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) StartEndTag(tag string, attrs []pyhtml.Attr) {
	r.events = append(r.events, testkit.Array(testkit.Str("se"), testkit.Str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) EndTag(tag string) {
	r.events = append(r.events, testkit.Array(testkit.Str("e"), testkit.Str(tag)))
}
func (r *eventRecorder) Data(text string) {
	r.events = append(r.events, testkit.Array(testkit.Str("d"), testkit.Str(text)))
}

func goEvents(page string) pyjson.Value {
	r := &eventRecorder{}
	pyhtml.Parse(page, r)
	return testkit.Array(r.events...)
}

// goFeedEvents are the callbacks of pyhtml.Feed (feed() without close()).
func goFeedEvents(page string) pyjson.Value {
	r := &eventRecorder{}
	pyhtml.Feed(page, r)
	return testkit.Array(r.events...)
}

// goPageLine is one page as [events, title, text].
func goPageLine(page string) string {
	title, text := HTMLToText(page)
	return pyjson.Dumps(testkit.Array(goEvents(page), testkit.Str(title), testkit.Str(text)), DumpDefault)
}

// htmlCorners are hand-written pages: the Python tests' fixtures and the
// tokenizer corners (unterminated constructs at the end, raw-text elements,
// attribute quoting, charrefs in data and attributes).
var htmlCorners = []string{
	"",
	pyTestPage,
	"<p>by expanding<b>Chart</b> palette</p>",
	"<table><tr><td>obj</td><td>the literal<table><tr><td>a</td><td>b</td></tr></table> end</td></tr></table>",
	"<table><tr><th>Name<th>Type\n<tr><td>port<td>int\n<tr><td>host<td>String</table>",
	"<p>before</p><table><tr><td>last cell text",
	"<html><title>Changes</title><p>html page words</p>",
	"a < b > c", "x <", "x </", "x </a", "x <a href='y", "x <!-- open", "x <!-->y", "x <!--->y",
	"x <!-- a --!> y", "x <? pi", "x <!doctype", "x <![CDATA[ a", "x <![CDATA[ a ]]> y", "x <!bogus> y",
	"x </1 bogus> y", "x </> y", "<script>a < b && c</script>d", "<script>never closed",
	"<script>x</script", "<script>x</scriptx></script >y", "<SCRIPT>x</Script>y", "<style>p{}</STYLE>z",
	"<title>A &amp; <b>B</b></title>", "<textarea>&lt;t&gt;</textarea>", "<plaintext><b>&amp;</b>",
	"<xmp><b></xmp>", "<img alt=\"&amp;x&ampy&lt=&#65;&#x42&notit;&notin;\" src=' s '>",
	"<img alt=a alt=b src>", "<img alt= 'unclosed src=x>", "<a x='1'/>", "<a//>", "<a / / >", "<a =x>",
	"<a\x0b>t</a\x0b>", "<br/>a<hr/>b<img src=c/>d<p/>e", "<pre>  a\r\n  b\n</pre>", "<pre><pre>x</pre>y</pre>",
	"</pre>a", "<h1></h1><h1>Second</h1>", "<h1>&#1;</h1><h1>X</h1>", "<p>Index | Prev | Next</p>",
	"<p>Index | Prev | Next</p>", "a\x00b<b>c</b>\x00d", "K<b>²</b>_", "<li>one<li>two",
	"&#8212;&rsquo;&nbsp&notit;&#xD800;&#128;&#0;&amp", "<td>|</td>", "<table><tr><td>a|b</td></tr></table>",
	"<table></table>", "<table><tr></tr><tr><td></td></tr></table>", "<tr><td>no table</td></tr>",
	"<pre>\n\n\ncode\n\n</pre>\n\n\n\ntext", "<p>```</p>a<p>```</p>  b  ",
}

func TestHTMLCornersFrozen(t *testing.T) {
	var got []string
	for _, page := range htmlCorners {
		got = append(got, goPageLine(page))
	}
	testkit.CheckFrozen(t, "html/corners", got)
}

// feedCorners are pages whose end feed() holds back, on top of htmlCorners:
// cut references inside and outside the 34-code-point look-back (counted in
// code points, so multi-byte text before the "&" matters), raw text without
// its end tag, a lone "<" or "</".
var feedCorners = []string{
	"tail &amp", "tail &amp x", "a&b" + strings.Repeat("\u00e9", 32), "a&b" + strings.Repeat("\u00e9", 33),
	"x &" + strings.Repeat("y", 31), "x &" + strings.Repeat("y", 32), "x &" + strings.Repeat("y", 40),
	"<script>x</scr", "<textarea>a &amp b", "<plaintext>a &amp", "a <", "a </", "a < b", "a&#65", "a&#65;",
	"<b>int</b>&nbsp;x", "a &\t", "a &;", "<p>a</p>&\u00e9\u00e9",
}

// TestHTMLFeedCornersFrozen: pyhtml.Feed against one feed() without
// close() (the strip_tags of bajadoc_parser) on the corner pages.
func TestHTMLFeedCornersFrozen(t *testing.T) {
	var got []string
	for _, page := range append(append([]string{}, htmlCorners...), feedCorners...) {
		got = append(got, pyjson.Dumps(goFeedEvents(page), DumpDefault))
	}
	testkit.CheckFrozen(t, "html/feed-corners", got)
}

// TestHTML5EntitiesFrozen: testdata/frozen/html/entities.txt holds, per
// key of html.entities.html5 (CPython 3.15), html.unescape of "a&KEY b",
// "&KEY=" and "&KEY&KEY". pyhtml.IsHTML5Entity must hold every key and no
// near miss of one, and pyhtml.Unescape must give the frozen texts.
func TestHTML5EntitiesFrozen(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "frozen", "html", "entities.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		v, err := pyjson.Loads(pyjson.Runes(line))
		if err != nil || v.Kind != pyjson.Array || len(v.A) != 4 {
			t.Fatalf("entities.txt line %q: %v", line, err)
		}
		keys = append(keys, v.A[0].S)
	}
	if len(keys) < 2000 {
		t.Fatalf("only %d html5 entities", len(keys))
	}
	in := map[string]bool{}
	for _, k := range keys {
		in[k] = true
	}
	bad := 0
	for _, k := range keys {
		base := strings.TrimSuffix(k, ";")
		for _, p := range []string{k, base, base + ";", base + "x", base + "x;", base[:len(base)-1], base[:len(base)-1] + ";"} {
			if p == "" || p == ";" || !isAlnumName(p) {
				continue
			}
			if got := pyhtml.IsHTML5Entity(p); got != in[p] {
				bad++
				if bad <= 10 {
					t.Errorf("IsHTML5Entity(%q) = %v, html5 has it: %v", p, got, in[p])
				}
			}
		}
	}
	var got []string
	for _, k := range keys {
		got = append(got, testkit.DumpLine(testkit.Str(k), testkit.Str(pyhtml.Unescape("a&"+k+" b")), testkit.Str(pyhtml.Unescape("&"+k+"=")),
			testkit.Str(pyhtml.Unescape("&"+k+"&"+k))))
	}
	testkit.CheckFrozen(t, "html/entities", got)
}

// isAlnumName: the attr_charref name shape, [a-zA-Z][a-zA-Z0-9]*;?.
func isAlnumName(p string) bool {
	p = strings.TrimSuffix(p, ";")
	letter := func(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }
	if p == "" || !letter(p[0]) {
		return false
	}
	for i := 1; i < len(p); i++ {
		if !letter(p[i]) && (p[i] < '0' || p[i] > '9') {
			return false
		}
	}
	return true
}
