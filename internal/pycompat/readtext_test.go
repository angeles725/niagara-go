package pycompat

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadText(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	newlines := write("newlines.md", "a\r\nb\rc\nñ\v\u0085")
	invalid := write("invalid.md", "ok\xff\r\nend")

	tests := []struct {
		name, path        string
		strict, replace   string
		strictErr, repErr error
	}{
		{"universal newlines only touch CR and CRLF", newlines, "a\nb\nc\nñ\v\u0085", "a\nb\nc\nñ\v\u0085", nil, nil},
		{"invalid UTF-8 is an error strictly and U+FFFD with replace", invalid, "", "ok�\nend", ErrNotUTF8, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadText(tt.path)
			if got != tt.strict || !errors.Is(err, tt.strictErr) {
				t.Errorf("ReadText = %q, %v; want %q, %v", got, err, tt.strict, tt.strictErr)
			}
			got, err = ReadTextReplace(tt.path)
			if got != tt.replace || !errors.Is(err, tt.repErr) {
				t.Errorf("ReadTextReplace = %q, %v; want %q, %v", got, err, tt.replace, tt.repErr)
			}
		})
	}
}

// TestReadTextStrerror pins the reason the corpus tools print for a failed read: the
// strerror without the "[Errno n] " prefix, and ErrNotUTF8's own text.
func TestReadTextStrerror(t *testing.T) {
	dir := t.TempDir()
	_, missing := ReadText(filepath.Join(dir, "absent.md"))
	_, isDir := ReadTextReplace(dir)
	_, notUTF8 := ReadText(func() string {
		p := filepath.Join(dir, "bad.md")
		if err := os.WriteFile(p, []byte("\xff"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}())
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"missing file", missing, "No such file or directory"},
		{"directory", isDir, "Is a directory"},
		{"error without errno keeps its text", notUTF8, "not valid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "directory" && runtime.GOOS == "windows" {
				t.Skip("reading a directory fails differently on Windows")
			}
			if got := StrerrorOf(tt.err); got != tt.want {
				t.Errorf("StrerrorOf(%v) = %q; want %q", tt.err, got, tt.want)
			}
		})
	}
}
