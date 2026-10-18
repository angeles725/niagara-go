package helpbuild

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Native fuzzing of the html.parser port and of html_text.convert: no
// input may panic, take longer than fuzzDeadline, or turn valid UTF-8 into
// output that is not. The seeds are the parity corner pages; the parser
// assumes valid UTF-8 (pages are read with errors="replace"), so a raw
// input is parsed as is (no panic, no hang) and then as
// strings.ToValidUTF8 for the output check. Run with
// go test -fuzz=FuzzParseHTML -fuzztime=120s ./internal/helpbuild/ (and
// FuzzHTMLToText); a crasher lands in testdata/fuzz/<Target>/.

const fuzzDeadline = 2 * time.Second

func fuzzSeeds(f *testing.F) {
	for _, page := range htmlCorners {
		f.Add(page)
	}
	f.Add("\xff<p \xfe=\x80>a</p\xc3>")
}

// withinDeadline runs fn and fails t when it does not return in time; a
// panic in fn surfaces in t as usual.
func withinDeadline(t *testing.T, page string, fn func()) {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		fn()
	}()
	select {
	case p := <-done:
		if p != nil {
			t.Fatalf("panic on %q: %v", page, p)
		}
	case <-time.After(fuzzDeadline):
		t.Fatalf("no result within %v on a %d-byte page %.200q", fuzzDeadline, len(page), page)
	}
}

// utf8Checker fails on any callback argument that is not valid UTF-8.
type utf8Checker struct {
	t    *testing.T
	page string
}

func (c utf8Checker) check(what, s string) {
	if !utf8.ValidString(s) {
		c.t.Errorf("%s %q of %q is not valid UTF-8", what, s, c.page)
	}
}

func (c utf8Checker) attrs(attrs []htmlAttr) {
	for _, a := range attrs {
		c.check("attribute name", a.Name)
		c.check("attribute value", a.Value)
	}
}

func (c utf8Checker) startTag(tag string, attrs []htmlAttr) { c.check("tag", tag); c.attrs(attrs) }
func (c utf8Checker) startEndTag(tag string, attrs []htmlAttr) {
	c.check("tag", tag)
	c.attrs(attrs)
}
func (c utf8Checker) endTag(tag string) { c.check("end tag", tag) }
func (c utf8Checker) data(text string)  { c.check("data", text) }

func FuzzParseHTML(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, raw string) {
		withinDeadline(t, raw, func() { parseHTML(raw, &eventRecorder{}) })
		page := strings.ToValidUTF8(raw, "�")
		withinDeadline(t, page, func() { parseHTML(page, utf8Checker{t, page}) })
	})
}

func FuzzHTMLToText(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, raw string) {
		withinDeadline(t, raw, func() { HTMLToText(raw) })
		page := strings.ToValidUTF8(raw, "�")
		var title, text string
		withinDeadline(t, page, func() { title, text = HTMLToText(page) })
		if !utf8.ValidString(title) || !utf8.ValidString(text) {
			t.Errorf("convert(%q) = %q, %q: not valid UTF-8", page, title, text)
		}
	})
}
