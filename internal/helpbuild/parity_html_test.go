package helpbuild

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Parity of the S5 tokenizer and converter with html.parser and
// html_text.convert, through testdata/parity/htmldriver.py.

func runHTMLDriver(t *testing.T, stdin string, args ...string) []string {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "htmldriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("htmldriver %v: %v\n%s", args, err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

// eventRecorder is htmldriver.py's _Recorder over pyhtml.Parse.
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

// goPageLine is one "pages" line ([events, title, text]), page first for
// "fuzz".
func goPageLine(page string, withPage bool) string {
	title, text := HTMLToText(page)
	vs := []pyjson.Value{goEvents(page), str(title), str(text)}
	if withPage {
		vs = append([]pyjson.Value{str(page)}, vs...)
	}
	return pyjson.Dumps(array(vs...), DumpDefault)
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

func TestParityHTMLCorners(t *testing.T) {
	driver(t)
	in, err := json.Marshal(htmlCorners)
	if err != nil {
		t.Fatal(err)
	}
	py := runHTMLDriver(t, string(in), "pages")
	var gol []string
	for _, page := range htmlCorners {
		gol = append(gol, goPageLine(page, false))
	}
	diffLines(t, "html corners", py, gol)
}

// TestParityHTMLFuzz compares seeded random snippets (htmldriver.py fuzz):
// 400 with -short, 5,000 otherwise.
func TestParityHTMLFuzz(t *testing.T) {
	driver(t)
	count := 5000
	if testing.Short() {
		count = 400
	}
	py := runHTMLDriver(t, "", "fuzz", "20261017", strconv.Itoa(count))
	if len(py) != count {
		t.Fatalf("driver returned %d cases, want %d", len(py), count)
	}
	gol := make([]string, len(py))
	for i, line := range py {
		v, err := pyjson.Loads(pyjson.Runes(line))
		if err != nil || v.Kind != pyjson.Array || len(v.A) == 0 {
			t.Fatalf("case %d: bad driver line %q: %v", i, line, err)
		}
		gol[i] = goPageLine(v.A[0].S, true)
	}
	diffLines(t, "html fuzz", py, gol)
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

// TestParityHTMLFeed compares pyhtml.Feed with one feed() without close()
// (the strip_tags of bajadoc_parser) on the corner pages and on seeded
// random snippets: 400 with -short, 5,000 otherwise.
func TestParityHTMLFeed(t *testing.T) {
	driver(t)
	pages := append(append([]string{}, htmlCorners...), feedCorners...)
	in, err := json.Marshal(pages)
	if err != nil {
		t.Fatal(err)
	}
	py := runHTMLDriver(t, string(in), "feed")
	var gol []string
	for _, page := range pages {
		gol = append(gol, pyjson.Dumps(goFeedEvents(page), DumpDefault))
	}
	diffLines(t, "feed corners", py, gol)
	count := 5000
	if testing.Short() {
		count = 400
	}
	py = runHTMLDriver(t, "", "feedfuzz", "20261018", strconv.Itoa(count))
	if len(py) != count {
		t.Fatalf("driver returned %d cases, want %d", len(py), count)
	}
	gol = make([]string, len(py))
	for i, line := range py {
		v, err := pyjson.Loads(pyjson.Runes(line))
		if err != nil || v.Kind != pyjson.Array || len(v.A) == 0 {
			t.Fatalf("case %d: bad driver line %q: %v", i, line, err)
		}
		gol[i] = pyjson.Dumps(array(v.A[0], goFeedEvents(v.A[0].S)), DumpDefault)
	}
	diffLines(t, "feed fuzz", py, gol)
}

// TestParityHTML5Entities checks pyhtml.IsHTML5Entity against every key of
// html.entities.html5 and their near misses, and pyhtml.Unescape against
// html.unescape over every key in data-like contexts.
func TestParityHTML5Entities(t *testing.T) {
	driver(t)
	var keys []string
	if err := json.Unmarshal([]byte(runHTMLDriver(t, "", "entities")[0]), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) < 2000 {
		t.Fatalf("only %d html5 entities", len(keys))
	}
	in := map[string]bool{}
	for _, k := range keys {
		in[k] = true
	}
	var probes []string
	for _, k := range keys {
		base := strings.TrimSuffix(k, ";")
		for _, p := range []string{k, base, base + ";", base + "x", base + "x;", base[:len(base)-1], base[:len(base)-1] + ";"} {
			if p != "" && p != ";" {
				probes = append(probes, p)
			}
		}
	}
	bad := 0
	for _, p := range probes {
		if !isAlnumName(p) {
			continue
		}
		if got := pyhtml.IsHTML5Entity(p); got != in[p] {
			bad++
			if bad <= 10 {
				t.Errorf("IsHTML5Entity(%q) = %v, html5 has it: %v", p, got, in[p])
			}
		}
	}
	var texts []string
	for _, p := range probes {
		texts = append(texts, "a&"+p+" b", "&"+p+"=", "&"+p+"&"+p)
	}
	data, err := json.Marshal(texts)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if err := json.Unmarshal([]byte(runHTMLDriver(t, string(data), "unescape")[0]), &want); err != nil {
		t.Fatal(err)
	}
	for i, s := range texts {
		if got := pyhtml.Unescape(s); got != want[i] {
			bad++
			if bad <= 20 {
				t.Errorf("Unescape(%q) = %q, html.unescape %q", s, got, want[i])
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d entity differences", bad)
	}
	t.Logf("%d probes, %d unescape texts identical", len(probes), len(texts))
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
