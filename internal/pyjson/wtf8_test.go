package pyjson

import (
	"slices"
	"testing"
)

// Runes keeps every valid UTF-8 sequence and every WTF-8 lone surrogate, and turns each byte
// that starts no valid sequence into one U+FFFD without swallowing the bytes after it (the
// decoder once read the next 1-3 bytes as continuation bytes whatever they were).
func TestRunesInvalidBytes(t *testing.T) {
	const fffd = 0xFFFD
	cases := []struct {
		in   string
		want []rune
	}{
		{"a\xffb", []rune{'a', fffd, 'b'}},
		{"\xc3(x", []rune{fffd, '(', 'x'}},
		{"\x80abc", []rune{fffd, 'a', 'b', 'c'}},
		{"\xe2\x82z", []rune{fffd, fffd, 'z'}},
		{"\xf0\x9f\x98", []rune{fffd, fffd, fffd}},
		{"\xc0\x80", []rune{fffd, fffd}},
		{"\xed\xa0\x80", []rune{0xD800}},
		{"\xed\xbf\xbf!", []rune{0xDFFF, '!'}},
		{"é€😀\uFFFD", []rune{'é', '€', '😀', fffd}},
	}
	for _, c := range cases {
		if got := Runes(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Runes(%q) = %U, want %U", c.in, got, c.want)
		}
	}
}

func TestEncodeStringInvalidByteKeepsFollowingText(t *testing.T) {
	if got, want := EncodeString("\xc3\"x", true), `"\ufffd\"x"`; got != want {
		t.Errorf("EncodeString = %s, want %s", got, want)
	}
}
