package modjar

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOpenListsNamesAndReadsByIndex(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jar")
	writeJar(t, p, entry{"META-INF/MANIFEST.MF", "m"}, entry{"a/B.class", "0123456789"}, entry{"a/B.class", "last"})
	j, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if got, want := j.Names(), []string{"META-INF/MANIFEST.MF", "a/B.class", "a/B.class"}; !slices.Equal(got, want) {
		t.Fatalf("Names = %q, want %q", got, want)
	}
	cases := []struct {
		name  string
		i     int
		limit int64
		want  string
	}{
		{"whole entry", 1, 0, "0123456789"},
		{"limit truncates", 1, 4, "0123"},
		{"limit above size", 1, 1 << 20, "0123456789"},
		{"duplicate keeps its own bytes", 2, 0, "last"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := j.Read(c.i, c.limit)
			if err != nil || string(b) != c.want {
				t.Fatalf("Read(%d, %d) = %q, %v; want %q", c.i, c.limit, b, err, c.want)
			}
		})
	}
	if got := j.Last("a/B.class"); got != 2 {
		t.Fatalf("Last = %d, want 2", got)
	}
	if got := j.Last("nope"); got != -1 {
		t.Fatalf("Last(absent) = %d, want -1", got)
	}
}

func TestOpenErrorsKeepTheirKind(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(filepath.Join(dir, "missing.jar")); !os.IsNotExist(err) {
		t.Fatalf("missing jar: err = %v, want os.IsNotExist", err)
	}
	bad := filepath.Join(dir, "bad.jar")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(bad); !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("not a zip: err = %v, want zip.ErrFormat", err)
	}
	crc := filepath.Join(dir, "crc.jar")
	corruptJar(t, crc)
	j, err := Open(crc)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := j.Read(0, 0); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("bad CRC: err = %v, want zip.ErrChecksum", err)
	}
}

// rawNameJar writes one entry whose stored name is raw (no UTF-8 flag), so a name function
// can decode it differently from f.Name.
func rawNameJar(t *testing.T, path, raw, body string) {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.CreateHeader(&zip.FileHeader{Name: raw, Method: zip.Store, NonUTF8: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWithNameFuncMatchesNormalizedNames(t *testing.T) {
	p := filepath.Join(t.TempDir(), "raw.jar")
	rawNameJar(t, p, "pal.palette\x00junk", "<palette/>")
	trimNUL := func(f *zip.File) string { n, _, _ := strings.Cut(f.Name, "\x00"); return n }

	if r := ReadMember(p, "pal.palette"); !errors.Is(r.Err, ErrNoMember) {
		t.Fatalf("raw name: err = %v, want ErrNoMember", r.Err)
	}
	r := ReadMember(p, "pal.palette", WithNameFunc(trimNUL))
	if r.Err != nil || string(r.Data) != "<palette/>" {
		t.Fatalf("normalized name: %q, %v", r.Data, r.Err)
	}
	j, err := Open(p, WithNameFunc(trimNUL))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if got := j.Names(); !slices.Equal(got, []string{"pal.palette"}) {
		t.Fatalf("Names = %q", got)
	}
}
