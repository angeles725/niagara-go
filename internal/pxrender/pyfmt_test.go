package pxrender

import (
	"math"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// parseFloat and splitextRoot adapt the pycompat calls render.go makes to one comparable value.
func parseFloat(s string) (float64, bool) {
	f, err := pycompat.ParseFloat(s)
	return f, err == nil
}

func splitextRoot(p string) string {
	root, _ := pypath.PosixSplitext(p)
	return root
}

// Every expected value below is Python 3 output.
func TestPyG(t *testing.T) {
	cases := map[float64]string{0: "0", 100: "100", 150: "150", 0.000123: "0.000123", 1234567: "1.23457e+06",
		100000: "100000", 1e6: "1e+06", 1e-5: "1e-05", 0.0001: "0.0001", 123456.5: "123456", 1.5: "1.5",
		2.675: "2.675", 1e21: "1e+21", -3.25: "-3.25", math.Inf(1): "inf"}
	for v, want := range cases {
		if got := pycompat.FormatG(v); got != want {
			t.Errorf("pycompat.FormatG(%v) = %q, want %q", v, got, want)
		}
	}
	if got := pycompat.FormatG(math.Copysign(0, -1)); got != "-0" {
		t.Errorf("pycompat.FormatG(-0) = %q", got)
	}
}

func TestPyFloat(t *testing.T) {
	ok := map[string]float64{" 1.5 ": 1.5, "1_000": 1000, "1e3": 1000, ".5": 0.5, "5.": 5,
		"+inf": math.Inf(1), "-Infinity": math.Inf(-1), "1e400": math.Inf(1)}
	for s, want := range ok {
		if got, good := parseFloat(s); !good || got != want {
			t.Errorf("parseFloat(%q) = %v, %v; want %v", s, got, good, want)
		}
	}
	for _, s := range []string{"nan", "-nan"} {
		if got, good := parseFloat(s); !good || !math.IsNaN(got) {
			t.Errorf("parseFloat(%q) = %v, %v; want nan", s, got, good)
		}
	}
	for _, s := range []string{"1__0", "_1", "0x10", "++1", "", "1,5", "0x1p3"} {
		if _, good := parseFloat(s); good {
			t.Errorf("parseFloat(%q) accepted; Python raises ValueError", s)
		}
	}
}

func TestPyUnescape(t *testing.T) {
	cases := map[string]string{"&#x;": "&#x;", "&#0;": "\uFFFD", "&#13;": "\r", "&#128;": "€", "&#x81;": "\u0081",
		"&#1;": "", "&#127;": "", "&#xfffe;": "", "&#x10ffff;": "", "&#99999999999;": "\uFFFD",
		"&#xD800;": "\uFFFD", "&notit;": "¬it;", "&amp-x": "&-x", "&ampx;": "&x;", "&eacute-x;": "é-x;",
		"&foo-bar;": "&foo-bar;", "a & b": "a & b", "&#169": "©", "&NotNestedGreaterGreater;": "⪢̸",
		"&lt": "<", "&acE;": "∾̳"}
	for in, want := range cases {
		if got := pyhtml.Unescape(in); got != want {
			t.Errorf("pyhtml.Unescape(%q) = %q, want %q", in, got, want)
		}
	}
	if got := pycompat.HTMLEscape(`<a href="x">it's & co</a>`); got != "&lt;a href=&quot;x&quot;&gt;it&#x27;s &amp; co&lt;/a&gt;" {
		t.Errorf("htmlEscape = %q", got)
	}
}

func TestPathHelpers(t *testing.T) {
	for in, want := range map[string]string{"a/b.px": "a/b", ".px": ".px", "..px": "..px", "a/.b.c": "a/.b",
		"noext": "noext", "a.b/c": "a.b/c"} {
		if got := splitextRoot(in); got != want {
			t.Errorf("splitextRoot(%q) = %q, want %q", in, got, want)
		}
	}
	if got := pypath.PosixJoin("shared", "/etc/x"); got != "/etc/x" {
		t.Errorf("pyJoin absolute = %q", got)
	}
	if got := pypath.PosixJoin("shared/", "px", "m", "a/../b"); got != "shared/px/m/a/../b" {
		t.Errorf("pyJoin = %q", got)
	}
}

func TestNormalizeAttrWhitespace(t *testing.T) {
	in := "<a t=\"x\r\ny\tz\nw\" u='p\rq'>text\nstays<!-- c\n --><![CDATA[\n]]></a>"
	want := "<a t=\"x y z w\" u='p q'>text\nstays<!-- c\n --><![CDATA[\n]]></a>"
	if got := string(normalizeAttrWhitespace([]byte(in))); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestParseXMLRejectsWhatExpatRejects(t *testing.T) {
	for _, doc := range []string{`<a x="1" x="2"/>`, `<a/><b/>`, `<a/>junk`, ``, `junk<a/>`, `<a>`} {
		if _, err := parseXML([]byte(doc)); err == nil {
			t.Errorf("parseXML(%q) accepted", doc)
		}
	}
	root, err := parseXML([]byte("\xef\xbb\xbf<a><!-- c --><?pi x?><b n='1'/></a>"))
	if err != nil || len(root.children) != 1 || root.children[0].attr("n") != "1" {
		t.Fatalf("root %+v, err %v", root, err)
	}
}

func TestRelayNumAndFont(t *testing.T) {
	for in, want := range map[string]string{"slot:Relay$5b7$5d$2eBO": "7", "slot:Relay$5b007$5d": "7",
		"slot:R[0]": "0", "slot:Fan": "", "": ""} {
		if got := relayNum(in); got != want {
			t.Errorf("relayNum(%q) = %q, want %q", in, got, want)
		}
	}
	if f := parseNiagaraFont("bold 26.0pt Arial"); f.weight != "bold" || f.size != "26" || f.family != "Arial" {
		t.Errorf("parseNiagaraFont = %+v", f)
	}
	if out := strings.Join(textfold.Fields("a\x1cb"), "|"); out != "a|b" {
		t.Errorf("pySplit = %q", out)
	}
}
