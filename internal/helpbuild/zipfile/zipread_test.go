package zipfile

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// readAll opens jar and reads every entry, whole or bounded; an entry that
// fails is reported as "!" + its error class.
func readAllEntries(t *testing.T, jar string, whole bool) []string {
	t.Helper()
	z, err := Open(jar)
	if err != nil {
		t.Fatalf("open %s: %v", jar, err)
	}
	defer z.f.Close()
	var out []string
	for i := range z.Infos {
		data, err := z.ReadEntry(&z.Infos[i], whole)
		if err != nil {
			out = append(out, z.Infos[i].Name+"!"+errClass(err))
			continue
		}
		out = append(out, z.Infos[i].Name+"="+string(data))
	}
	return out
}

func errClass(err error) string {
	switch {
	case errors.Is(err, errEncryptedEntry):
		return "encrypted"
	case errors.Is(err, errUnsupportedMethod), errors.Is(err, errUnsupportedZip):
		return "unsupported"
	case errors.Is(err, ErrTruncatedEntry):
		return "truncated"
	case errors.Is(err, ErrBadZip):
		return "bad"
	}
	return "other: " + err.Error()
}

// Stored and deflated entries come back byte for byte, empty ones too.
func TestReadEntryStoredAndDeflated(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "m.jar")
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	big := strings.Repeat("deflate me ", 5000)
	for _, f := range []struct {
		name, data string
		method     uint16
	}{{"a.txt", "stored", zip.Store}, {"b.txt", big, zip.Deflate}, {"e.txt", "", zip.Deflate}, {"s.txt", "", zip.Store}} {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f.name, Method: f.method})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(f.data))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, jar, buf.Bytes())
	for _, whole := range []bool{true, false} {
		got := readAllEntries(t, jar, whole)
		want := []string{"a.txt=stored", "b.txt=" + big, "e.txt=", "s.txt="}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("whole=%v: %.200q", whole, got)
		}
	}
}

// The ZipFile.open / ZipExtFile corners (each one checked against Python
// while it was the oracle; TestDocJarCornersFrozen replays the library calls).
func TestReadEntryFollowsPythonZipfile(t *testing.T) {
	cases := []struct {
		name    string
		entries []testkit.RawEntry
		want    []string
	}{
		{"bad CRC", []testkit.RawEntry{{Name: "a.txt", Data: "abc", CRCXor: 1}}, []string{"a.txt!bad"}},
		{"local name differs", []testkit.RawEntry{{Name: "a.txt", Data: "abc", LocalName: "b.txt"}}, []string{"a.txt!bad"}},
		{"cp437 local name equals the decoded central name", []testkit.RawEntry{{Name: "\x81.txt", Data: "u"}}, []string{"ü.txt=u"}},
		{"encrypted", []testkit.RawEntry{{Name: "a.txt", Data: "abc", Flags: 1}}, []string{"a.txt!encrypted"}},
		{"strong encryption", []testkit.RawEntry{{Name: "a.txt", Data: "abc", Flags: 0x40}}, []string{"a.txt!unsupported"}},
		{"bzip2 method", []testkit.RawEntry{{Name: "a.txt", Data: "abc", Method: 12}}, []string{"a.txt!unsupported"}},
		{"unknown method", []testkit.RawEntry{{Name: "a.txt", Data: "abc", Method: 99}}, []string{"a.txt!unsupported"}},
		{"file_size shorter than the data", []testkit.RawEntry{{Name: "a.txt", Data: "hello", ShortSize: 3}}, []string{"a.txt=hel"}},
		{"compress_size into the next header", []testkit.RawEntry{{Name: "a.txt", Data: "abc", CompPlus: 4}, {Name: "b.txt", Data: "b"}},
			[]string{"a.txt!bad", "b.txt=b"}},
		{"duplicate sharing one local header", []testkit.RawEntry{{Name: "a.txt", Data: "abc"}, {Name: "a.txt", Data: "abc", SharedHeader: 1}},
			[]string{"a.txt=abc", "a.txt=abc"}},
		{"NUL-cut name keeps its original for the header check", []testkit.RawEntry{{Name: "a.txt\x00.exe", Data: "n"}}, []string{"a.txt=n"}},
	}
	dir := t.TempDir()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jar := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+".jar")
			testkit.WriteFile(t, jar, testkit.RawZip("", c.entries, ""))
			for _, whole := range []bool{true, false} {
				if got := readAllEntries(t, jar, whole); strings.Join(got, "|") != strings.Join(c.want, "|") {
					t.Fatalf("whole=%v: %q, want %q", whole, got, c.want)
				}
			}
		})
	}
}

// Prepended data shifts every local header (concat), the reader follows.
func TestReadEntryAfterPrependedData(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "p.jar")
	testkit.WriteFile(t, jar, testkit.RawZip(strings.Repeat("#", 77), []testkit.RawEntry{{Name: "a.txt", Data: "abc"}, {Name: "b.txt", Data: "de"}}, ""))
	if got := readAllEntries(t, jar, true); strings.Join(got, "|") != "a.txt=abc|b.txt=de" {
		t.Fatalf("%q", got)
	}
	if _, err := Open(filepath.Join(t.TempDir(), "missing.jar")); !os.IsNotExist(err) {
		t.Fatalf("missing jar: %v", err)
	}
}

// A deflated entry whose compressed bytes stop before compress_size is
// EOFError in Python (ZipExtFile._read2 reads nothing while bytes are
// still owed), for read() and copyfileobj's bounded reads alike: the same
// ErrTruncatedEntry as a short stored entry. The file ends inside the
// entry's data; the Info is built by hand.
func TestReadEntryTruncatedDeflate(t *testing.T) {
	data := bytes.Repeat([]byte("truncated deflate "), 2000)
	var comp bytes.Buffer
	fw, err := flate.NewWriter(&comp, 9)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	var hdr bytes.Buffer
	for _, v := range []any{[]byte("PK\x03\x04"), uint16(20), uint16(0), uint16(zipDeflated), uint32(0),
		crc32.ChecksumIEEE(data), uint32(comp.Len()), uint32(len(data)), uint16(5), uint16(0), []byte("a.txt")} {
		_ = binary.Write(&hdr, binary.LittleEndian, v)
	}
	for _, cut := range []int{0, 1, comp.Len() / 2} {
		path := filepath.Join(t.TempDir(), "t.bin")
		testkit.WriteFile(t, path, append(append([]byte{}, hdr.Bytes()...), comp.Bytes()[:cut]...))
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := f.Stat()
		z := &File{f: f, size: st.Size()}
		x := &Info{Name: "a.txt", origName: "a.txt", Size: uint64(len(data)), CRC: crc32.ChecksumIEEE(data),
			method: zipDeflated, compressSize: uint64(comp.Len())}
		for _, whole := range []bool{true, false} {
			if _, err := z.ReadEntry(x, whole); !errors.Is(err, ErrTruncatedEntry) {
				t.Errorf("cut %d whole=%v: %v, want ErrTruncatedEntry", cut, whole, err)
			}
		}
		f.Close()
	}
}
