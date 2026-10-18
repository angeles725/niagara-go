package helpbuild

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

// TestParityHTML5Entities checks isHTML5Entity against every key of
// html.entities.html5 and their near misses, and htmlUnescape
// (pycompat.HTMLUnescape plus the names Go lacks) against html.unescape over
// every key in data-like contexts.
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
		if got := isHTML5Entity(p); got != in[p] {
			bad++
			if bad <= 10 {
				t.Errorf("isHTML5Entity(%q) = %v, html5 has it: %v", p, got, in[p])
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
		if got := htmlUnescape(s); got != want[i] {
			bad++
			if bad <= 20 {
				t.Errorf("HTMLUnescape(%q) = %q, html.unescape %q", s, got, want[i])
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
	if p == "" || !isASCIILetter(p[0]) {
		return false
	}
	for i := 1; i < len(p); i++ {
		if !isASCIILetter(p[i]) && !isDigitByte(p[i]) {
			return false
		}
	}
	return true
}
