package pyhtml

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Native fuzzing of the tokenizer (Parse and Feed): no input may panic, take longer than
// fuzzDeadline, or turn valid UTF-8 into a callback argument that is not.
// The parser assumes valid UTF-8 (the tools read pages with
// errors="replace"), so a raw input is parsed as is (no panic, no hang) and
// then as strings.ToValidUTF8 for the output check. Run with
// go test -fuzz=FuzzParse -fuzztime=120s ./internal/pyhtml/; a crasher
// lands in testdata/fuzz/FuzzParse/. helpbuild's FuzzHTMLToText fuzzes the
// converter on top of it.

const fuzzDeadline = 2 * time.Second

// withinDeadline runs fn and fails t when it does not return in time. fn
// reports through its return value, never through t: a run past the
// deadline keeps going after t has failed, so only the goroutine that owns
// t calls it. A panic in fn fails t too.
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

func (c *utf8Checker) attrs(attrs []Attr) {
	for _, a := range attrs {
		c.check("attribute name", a.Name)
		c.check("attribute value", a.Value)
	}
}

func (c *utf8Checker) StartTag(tag string, attrs []Attr)    { c.check("tag", tag); c.attrs(attrs) }
func (c *utf8Checker) StartEndTag(tag string, attrs []Attr) { c.check("tag", tag); c.attrs(attrs) }
func (c *utf8Checker) EndTag(tag string)                    { c.check("end tag", tag) }
func (c *utf8Checker) Data(text string)                     { c.check("data", text) }

func FuzzParse(f *testing.F) {
	for _, c := range append(append([]struct{ page, want string }{}, eventCases...), feedCases...) {
		f.Add(c.page)
	}
	f.Add("\xff<p \xfe=\x80>a</p\xc3>")
	f.Fuzz(func(t *testing.T, raw string) {
		withinDeadline(t, raw, func() []string { Parse(raw, &eventRecorder{}); return nil })
		withinDeadline(t, raw, func() []string { Feed(raw, &eventRecorder{}); return nil })
		page := strings.ToValidUTF8(raw, "\uFFFD")
		withinDeadline(t, page, func() []string {
			c := &utf8Checker{page: page}
			Parse(page, c)
			Feed(page, c)
			return c.problems
		})
	})
}
