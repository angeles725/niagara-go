package bog

import (
	"testing"
	"unicode"
)

// CPython's UTF-8 'replace' decoder: one U+FFFD per maximal invalid subpart (not per byte).
func TestDecodeReplaceMatchesPython(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"\xff", "\ufffd"},
		{"a\xe2\x82b", "a\ufffdb"},
		{"\xed\xa0\x80", "\ufffd\ufffd\ufffd"},
		{"\xf0\x9f\x98", "\ufffd"},
		{"\xc0\xaf", "\ufffd\ufffd"},
		{"\xf4\x90\x80\x80", "\ufffd\ufffd\ufffd\ufffd"},
		{"\xf8\x88\x80\x80\x80", "\ufffd\ufffd\ufffd\ufffd\ufffd"},
		{"\xe0\x80\xaf", "\ufffd\ufffd\ufffd"},
		{"\xc3", "\ufffd"},
		{"\x80\x80", "\ufffd\ufffd"},
		{"\xf0\x80\x80\x80", "\ufffd\ufffd\ufffd\ufffd"},
		{"\xf0\x9f\x98\x80\xf0\x9f", "\U0001F600\ufffd"},
		{"é€😀", "é€😀"},
	}
	for _, c := range cases {
		if got := decodeReplace([]byte(c.in)); got != c.want {
			t.Errorf("decodeReplace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPyReprMatchesPython(t *testing.T) {
	cases := []struct{ in, want string }{
		{"file.xml", "'file.xml'"},
		{"it's", `"it's"`},
		{`a'b"c`, `'a\'b"c'`},
		{`C:\x\file.xml`, `'C:\\x\\file.xml'`},
		{"t\tn\nr\r", `'t\tn\nr\r'`},
		{"\x00\x7f", `'\x00\x7f'`},
		{"é\u200b\U000E0001", `'é\u200b\U000e0001'`},
	}
	for _, c := range cases {
		if got := pyRepr(c.in); got != c.want {
			t.Errorf("pyRepr(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// Python 3.15 str.isspace() and str.splitlines() boundaries (Unicode 17, same as Go's tables).
func TestPythonCharacterClasses(t *testing.T) {
	space := map[rune]bool{}
	for _, r := range []rune{0x9, 0xa, 0xb, 0xc, 0xd, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x85, 0xa0, 0x1680,
		0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a,
		0x2028, 0x2029, 0x202f, 0x205f, 0x3000} {
		space[r] = true
	}
	lineBreak := map[rune]bool{}
	for _, r := range []rune{0xa, 0xb, 0xc, 0xd, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029} {
		lineBreak[r] = true
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if got := isPySpace(r); got != space[r] {
			t.Fatalf("isPySpace(%U) = %v", r, got)
		}
		if got := isLineBreak(r); got != lineBreak[r] {
			t.Fatalf("isLineBreak(%U) = %v", r, got)
		}
		if got, want := isWordRune(r), unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'; got != want {
			t.Fatalf("isWordRune(%U) = %v", r, got)
		}
	}
}

func TestPyFields(t *testing.T) {
	got := pyFields("b=baja\x1cc=control\u00a0 d=x\x1f")
	want := []string{"b=baja", "c=control", "d=x"}
	if len(got) != len(want) {
		t.Fatalf("pyFields = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pyFields = %q", got)
		}
	}
}
