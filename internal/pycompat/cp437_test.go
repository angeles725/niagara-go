package pycompat

import "testing"

func TestDecodeCP437(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"", ""},
		{"META-INF/module.xml", "META-INF/module.xml"},
		{"\x80", "Ç"},
		{"stra\xe1e.txt", "straße.txt"},
		{"\x9e\xa4\xff", "₧ñ "},
		{"\x7f\x00", "\x7f\x00"},
		// Already UTF-8 bytes are still read one byte at a time, as bytes.decode("cp437") does.
		{"\xc3\xa9", "├⌐"},
	} {
		if got := DecodeCP437(tt.in); got != tt.want {
			t.Errorf("DecodeCP437(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeCodepageDecodesEveryByte(t *testing.T) {
	if len(CP437Upper) != 128 {
		t.Fatalf("CP437Upper has %d runes, want 128", len(CP437Upper))
	}
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	got := []rune(DecodeCodepage(string(all), CP437Upper))
	if len(got) != 256 {
		t.Fatalf("256 bytes decode to %d runes", len(got))
	}
	for i, r := range got {
		want := rune(i)
		if i >= 0x80 {
			want = CP437Upper[i-0x80]
		}
		if r != want {
			t.Errorf("byte 0x%02x = %U, want %U", i, r, want)
		}
	}
}
