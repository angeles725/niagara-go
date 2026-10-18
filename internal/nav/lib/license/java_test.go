package license

import (
	"reflect"
	"testing"
)

// TestCallArgsAndCommentSpans pins java_text.call_args and comment_spans
// (values printed by Python on the same texts).
func TestCallArgsAndCommentSpans(t *testing.T) {
	for _, c := range []struct {
		text string
		want []string
		ok   bool
	}{
		{`f(a, "b,)", g(c, d))`, []string{"a", `"b,)"`, "g(c, d)"}, true},
		{`f( )`, []string{}, true},
		{`f(a, [1, 2], {x})`, []string{"a", "[1, 2]", "{x}"}, true},
		{`f(a, "unterminated`, nil, false},
		{`f(a, b`, nil, false},
	} {
		got, ok := callArgs(c.text, 1)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("callArgs(%q) = %q %v, want %q %v", c.text, got, ok, c.want, c.ok)
		}
	}
	text := "a // x\n\"//no\" /* y */ '/' /* open"
	want := []Span{{2, 6}, {14, 21}, {26, len(text)}}
	if got := CommentSpans(text); !reflect.DeepEqual(got, want) {
		t.Errorf("commentSpans = %v, want %v", got, want)
	}
}
