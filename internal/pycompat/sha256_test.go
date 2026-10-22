package pycompat

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content []byte // nil: the file is not created
		want    string
	}{
		{"empty file", []byte{}, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc vector", []byte("abc"), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"missing file", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			if tt.content != nil {
				if err := os.WriteFile(path, tt.content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := FileSHA256(path)
			if tt.content == nil {
				if !errors.Is(err, fs.ErrNotExist) || got != "" {
					t.Fatalf("FileSHA256(missing) = %q, %v; want \"\", ErrNotExist", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("FileSHA256 = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
