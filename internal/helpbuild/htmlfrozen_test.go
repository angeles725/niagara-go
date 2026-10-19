package helpbuild

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The S5 tokenizer and converter (pyhtml.Parse, pyhtml.Feed, HTMLToText)
// and the html5 entity table against their frozen Python output
// (frozen_test.go): html.parser's callbacks and html_text.convert on the
// corner pages, and one feed() without close() on them and on the feed
// corners.

// eventRecorder records the pyhtml callbacks as html.parser's would be.
type eventRecorder struct{ events []pyjson.Value }

func attrsValue(attrs []pyhtml.Attr) pyjson.Value {
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

func (r *eventRecorder) StartTag(tag string, attrs []pyhtml.Attr) {
	r.events = append(r.events, array(str("s"), str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) StartEndTag(tag string, attrs []pyhtml.Attr) {
	r.events = append(r.events, array(str("se"), str(tag), attrsValue(attrs)))
}
func (r *eventRecorder) EndTag(tag string) { r.events = append(r.events, array(str("e"), str(tag))) }
func (r *eventRecorder) Data(text string)  { r.events = append(r.events, array(str("d"), str(text))) }

func goEvents(page string) pyjson.Value {
	r := &eventRecorder{}
	pyhtml.Parse(page, r)
	return array(r.events...)
}

// goFeedEvents are the callbacks of pyhtml.Feed (feed() without close()).
func goFeedEvents(page string) pyjson.Value {
	r := &eventRecorder{}
	pyhtml.Feed(page, r)
	return array(r.events...)
}

// goPageLine is one page as [events, title, text].
func goPageLine(page string) string {
	title, text := HTMLToText(page)
	return pyjson.Dumps(array(goEvents(page), str(title), str(text)), DumpDefault)
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
	checkFrozen(t, "html/corners", got)
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
	checkFrozen(t, "html/feed-corners", got)
}
