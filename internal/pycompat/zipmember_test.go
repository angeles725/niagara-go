package pycompat

import (
	"archive/zip"
	"bytes"
	"testing"
)

// zipFiles is the entry list of an archive written in-test, members in order (names may repeat).
func zipFiles(t *testing.T, members ...[2]string) []*zip.File {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, m := range members {
		f, err := w.Create(m[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(m[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return r.File
}

func TestReadZipMemberLastEntryWins(t *testing.T) {
	files := zipFiles(t, [2]string{"a.txt", "first"}, [2]string{"b.txt", "other"}, [2]string{"a.txt", "second"})
	named := func(name string) func(*zip.File) bool {
		return func(f *zip.File) bool { return f.Name == name }
	}
	if data, ok := ReadZipMember(files, named("a.txt")); !ok || string(data) != "second" {
		t.Errorf("repeated member = %q %v, want the last entry", data, ok)
	}
	if data, ok := ReadZipMember(files, named("b.txt")); !ok || string(data) != "other" {
		t.Errorf("single member = %q %v", data, ok)
	}
	if data, ok := ReadZipMember(files, named("missing")); ok || data != nil {
		t.Errorf("missing member = %q %v, want nil false", data, ok)
	}
}

func TestReadZipFileFailsOnACorruptEntry(t *testing.T) {
	files := zipFiles(t, [2]string{"a.txt", "body"})
	if data, ok := ReadZipFile(files[0]); !ok || string(data) != "body" {
		t.Fatalf("ReadZipFile = %q %v", data, ok)
	}
	files[0].Method = 99 // an unknown compression method: Open fails
	if _, ok := ReadZipFile(files[0]); ok {
		t.Error("an unreadable entry read")
	}
}
