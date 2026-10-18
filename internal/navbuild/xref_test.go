package navbuild

import (
	"reflect"
	"strings"
	"testing"
)

func TestFindImports(t *testing.T) {
	text := "package p;\n\nimport a.B;\n  import static c.D.e ;\n\n \t\nimport f.*;\nimport static;\nimport g.h.; import i.J;\n" +
		"// import k.L;\nimportx m.N;\nimport static o.P.*;\nimport static.q;\nimport café.Été;\nimport r.S"
	want := []importMatch{{false, "a.B"}, {true, "c.D.e"}, {false, "f.*"}, {false, "static"}, {false, "g.h."},
		{true, "o.P.*"}, {false, "static.q"}, {false, "café.Été"}}
	if got := findImports(text); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestFirstLines(t *testing.T) {
	for _, tc := range []struct {
		content string
		n       int
		want    string
	}{
		{"a\nb\nc", 2, "a\nb"},
		{"a\nb\n", 2, "a\nb"},
		{"a\nb", 2, "a\nb"},
		{"a", 2, "a"},
		{"", 2, ""},
		{strings.Repeat("x\n", 3), 3, "x\nx\nx"},
	} {
		if got := firstLines(tc.content, tc.n); got != tc.want {
			t.Errorf("firstLines(%q, %d) = %q, want %q", tc.content, tc.n, got, tc.want)
		}
	}
}
