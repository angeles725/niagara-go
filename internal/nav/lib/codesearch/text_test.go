package codesearch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadLinesIsPythonReadlines(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []string
	}{
		{"empty file", "", []string{}},
		{"no final newline", "a\nb", []string{"a\n", "b"}},
		{"blank lines kept", "a\n\n", []string{"a\n", "\n"}},
		{"crlf and lone cr are newlines", "a\r\nb\rc\n", []string{"a\n", "b\n", "c\n"}},
		{"other line breaks are not split", "a\vb\x1cc\u2028d\n", []string{"a\vb\x1cc\u2028d\n"}},
		{"invalid utf-8 becomes U+FFFD", "x\xffy\xe2\x82\n", []string{"x\ufffdy\ufffd\n"}},
		{"bom kept (utf-8, not utf-8-sig)", "\ufeffclass A\n", []string{"\ufeffclass A\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "A.java")
			if err := os.WriteFile(p, []byte(tt.data), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := ReadLines(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadLines = %q, want %q", got, tt.want)
			}
		})
	}
	if _, err := ReadLines(filepath.Join(t.TempDir(), "missing.java")); err == nil {
		t.Error("ReadLines of a missing file: no error")
	}
}

func TestRStripAndEllipsis(t *testing.T) {
	if got := RStrip("x = 1;\u00a0\x1f\u2028 \t\n"); got != "x = 1;" {
		t.Errorf("RStrip = %q", got)
	}
	if got := RStrip("\u200bx\u200b"); got != "\u200bx\u200b" { // not whitespace for Python
		t.Errorf("RStrip(zero-width space) = %q", got)
	}
	tests := []struct {
		s    string
		max  int
		want string
	}{
		{"abcdef", 6, "abcdef"},
		{"abcdefg", 6, "abc..."},
		{"ñññññ", 5, "ñññññ"}, // 10 bytes, 5 code points: not cut
		{"ññññññ", 5, "ññ..."},
	}
	for _, tt := range tests {
		if got := Ellipsis(tt.s, tt.max); got != tt.want {
			t.Errorf("Ellipsis(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
	}
}
