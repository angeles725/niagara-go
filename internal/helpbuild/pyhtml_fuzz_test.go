package helpbuild

import (
	"fmt"
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

// withinDeadline runs fn and fails t when it does not return in time. fn
// reports through its return value, never through t: a run past the
// deadline keeps going after t has failed (and its test may have ended),
// so only the goroutine that owns t calls it. A panic in fn fails t too.
func withinDeadline(t *testing.T, page string, fn func() []string) {
	t.Helper()
	type result struct {
		panicked any
		problems []string
	}
	done := make(chan result, 1) // buffered: a late send never blocks
	go func() {
		var r result
		defer func() {
			r.panicked = recover()
			done <- r
		}()
		r.problems = fn()
	}()
	select {
	case r := <-done:
		if r.panicked != nil {
			t.Fatalf("panic on %q: %v", page, r.panicked)
		}
		for _, p := range r.problems {
			t.Error(p)
		}
	case <-time.After(fuzzDeadline):
		t.Fatalf("no result within %v on a %d-byte page %.200q", fuzzDeadline, len(page), page)
	}
}

// utf8Checker collects every callback argument that is not valid UTF-8.
type utf8Checker struct {
	page     string
	problems []string
}

func (c *utf8Checker) check(what, s string) {
	if !utf8.ValidString(s) {
		c.problems = append(c.problems, fmt.Sprintf("%s %q of %q is not valid UTF-8", what, s, c.page))
	}
}

func (c *utf8Checker) attrs(attrs []htmlAttr) {
	for _, a := range attrs {
		c.check("attribute name", a.Name)
		c.check("attribute value", a.Value)
	}
}

func (c *utf8Checker) startTag(tag string, attrs []htmlAttr) { c.check("tag", tag); c.attrs(attrs) }
func (c *utf8Checker) startEndTag(tag string, attrs []htmlAttr) {
	c.check("tag", tag)
	c.attrs(attrs)
}
func (c *utf8Checker) endTag(tag string) { c.check("end tag", tag) }
func (c *utf8Checker) data(text string)  { c.check("data", text) }

func FuzzParseHTML(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, raw string) {
		withinDeadline(t, raw, func() []string { parseHTML(raw, &eventRecorder{}); return nil })
		page := strings.ToValidUTF8(raw, "�")
		withinDeadline(t, page, func() []string {
			c := &utf8Checker{page: page}
			parseHTML(page, c)
			return c.problems
		})
	})
}

func FuzzHTMLToText(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, raw string) {
		withinDeadline(t, raw, func() []string { HTMLToText(raw); return nil })
		page := strings.ToValidUTF8(raw, "�")
		withinDeadline(t, page, func() []string {
			title, text := HTMLToText(page)
			if !utf8.ValidString(title) || !utf8.ValidString(text) {
				return []string{fmt.Sprintf("convert(%q) = %q, %q: not valid UTF-8", page, title, text)}
			}
			return nil
		})
	})
}
