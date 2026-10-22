package core

import (
	"archive/zip"
	"testing"
)

func TestZipName(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags uint16
		want  string
	}{
		{"doc/a.html", 0, "doc/a.html"},
		{"stra\xe1e.txt", 0, "straße.txt"},
		{"stra\xc3\x9fe.txt", 0x800, "straße.txt"}, // UTF-8 flag: the name is kept as is
		{"a.txt\x00junk", 0, "a.txt"},
	} {
		f := &zip.File{FileHeader: zip.FileHeader{Name: tt.name, Flags: tt.flags}}
		if got := ZipName(f); got != tt.want {
			t.Errorf("ZipName(%q, %#x) = %q, want %q", tt.name, tt.flags, got, tt.want)
		}
	}
}
