package javatext

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCommentSpans pins java_text.comment_spans (values printed by Python on
// the same text).
func TestCommentSpans(t *testing.T) {
	text := "a // x\n\"//no\" /* y */ '/' /* open"
	want := []Span{{2, 6}, {14, 21}, {26, len(text)}}
	if got := CommentSpans(text); !reflect.DeepEqual(got, want) {
		t.Errorf("CommentSpans = %v, want %v", got, want)
	}
	if got := CommentSpans("no comments"); got != nil {
		t.Errorf("CommentSpans without comments = %v, want nil", got)
	}
}

func TestStripComments(t *testing.T) {
	for in, want := range map[string]string{
		`a /* x */ b // y`:            `a   b  `,
		`s = "// not" + '/' // c`:     `s = "// not" + '/'  `,
		"open /* never closed":        "open  ",
		`esc = "a\"//b" // c`:         `esc = "a\"//b"  `,
		"str ends at newline \"\n//c": "str ends at newline \"\n ",
		"no comments":                 "no comments",
	} {
		if got := StripComments(in); got != want {
			t.Errorf("StripComments(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeclarationHeader(t *testing.T) {
	lines := []string{
		"public class A<T> // trailing { brace in a comment\n",
		"   extends B /* { */ implements\n",
		"   C, D {\n",
	}
	if got, want := DeclarationHeader(lines, 0, len("public class A")), "<T> extends B implements C, D"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Without a brace the header stops after MaxHeaderLines lines.
	var long []string
	for i := 0; i < 50; i++ {
		long = append(long, "x\n")
	}
	if got := strings.Fields(DeclarationHeader(long, 0, 0)); len(got) != MaxHeaderLines {
		t.Errorf("unterminated header joined %d lines, want %d", len(got), MaxHeaderLines)
	}
	// A header near the end of the file stops at the last line.
	if got := DeclarationHeader([]string{"x\n", "y\n"}, 0, 0); got != "x y" {
		t.Errorf("header at end of file = %q, want %q", got, "x y")
	}
}

// TestRuneTests pins the \w and \s tests at a byte offset, Unicode classes
// and the ends of the text included.
func TestRuneTests(t *testing.T) {
	s := "é_  x" // é (2 bytes), _, space, NBSP (2 bytes), x
	for _, c := range []struct {
		i          int
		at, before bool
		rune       rune
		width      int
	}{
		{-1, false, false, utf8.RuneError, 0},
		{0, true, false, 'é', 2},
		{2, true, true, '_', 1},
		{3, false, true, ' ', 1},
		{4, false, false, ' ', 2},
		{6, true, false, 'x', 1},
		{7, false, true, utf8.RuneError, 0},
	} {
		if r, n := RuneAt(s, c.i); r != c.rune || n != c.width {
			t.Errorf("RuneAt(%d) = %q %d, want %q %d", c.i, r, n, c.rune, c.width)
		}
		if got := WordAt(s, c.i); got != c.at {
			t.Errorf("WordAt(%d) = %v, want %v", c.i, got, c.at)
		}
		if got := WordBefore(s, c.i); got != c.before {
			t.Errorf("WordBefore(%d) = %v, want %v", c.i, got, c.before)
		}
	}
	for _, c := range []struct{ i, want int }{{0, 0}, {3, 6}, {6, 6}, {7, 7}} {
		if got := SkipSpace(s, c.i); got != c.want {
			t.Errorf("SkipSpace(%d) = %d, want %d", c.i, got, c.want)
		}
	}
}
