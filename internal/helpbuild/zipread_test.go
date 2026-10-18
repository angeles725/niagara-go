package helpbuild

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
)

// readAll opens jar and reads every entry, whole or bounded; an entry that
// fails is reported as "!" + its error class.
func readAllEntries(t *testing.T, jar string, whole bool) []string {
	t.Helper()
	z, err := openZip(jar)
	if err != nil {
		t.Fatalf("open %s: %v", jar, err)
	}
	defer z.f.Close()
	var out []string
	for i := range z.infos {
		data, err := z.readEntry(&z.infos[i], whole)
		if err != nil {
			out = append(out, z.infos[i].name+"!"+errClass(err))
			continue
		}
		out = append(out, z.infos[i].name+"="+string(data))
	}
	return out
}

func errClass(err error) string {
	switch {
	case errors.Is(err, ErrEncryptedEntry):
		return "encrypted"
	case errors.Is(err, ErrUnsupportedMethod), errors.Is(err, ErrUnsupportedZip):
		return "unsupported"
	case errors.Is(err, errTruncatedEntry):
		return "truncated"
	case errors.Is(err, errBadZip):
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
	writeFile(t, jar, buf.Bytes())
	for _, whole := range []bool{true, false} {
		got := readAllEntries(t, jar, whole)
		want := []string{"a.txt=stored", "b.txt=" + big, "e.txt=", "s.txt="}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("whole=%v: %.200q", whole, got)
		}
	}
}

// The ZipFile.open / ZipExtFile corners (each one checked against Python by
// TestParityDocJarCorners).
func TestReadEntryFollowsPythonZipfile(t *testing.T) {
	cases := []struct {
		name    string
		entries []rawEntry
		want    []string
	}{
		{"bad CRC", []rawEntry{{name: "a.txt", data: "abc", crcXor: 1}}, []string{"a.txt!bad"}},
		{"local name differs", []rawEntry{{name: "a.txt", data: "abc", localName: "b.txt"}}, []string{"a.txt!bad"}},
		{"cp437 local name equals the decoded central name", []rawEntry{{name: "\x81.txt", data: "u"}}, []string{"ü.txt=u"}},
		{"encrypted", []rawEntry{{name: "a.txt", data: "abc", flags: 1}}, []string{"a.txt!encrypted"}},
		{"strong encryption", []rawEntry{{name: "a.txt", data: "abc", flags: 0x40}}, []string{"a.txt!unsupported"}},
		{"bzip2 method", []rawEntry{{name: "a.txt", data: "abc", method: 12}}, []string{"a.txt!unsupported"}},
		{"unknown method", []rawEntry{{name: "a.txt", data: "abc", method: 99}}, []string{"a.txt!unsupported"}},
		{"file_size shorter than the data", []rawEntry{{name: "a.txt", data: "hello", shortSize: 3}}, []string{"a.txt=hel"}},
		{"compress_size into the next header", []rawEntry{{name: "a.txt", data: "abc", compPlus: 4}, {name: "b.txt", data: "b"}},
			[]string{"a.txt!bad", "b.txt=b"}},
		{"duplicate sharing one local header", []rawEntry{{name: "a.txt", data: "abc"}, {name: "a.txt", data: "abc", sharedHeader: 1}},
			[]string{"a.txt=abc", "a.txt=abc"}},
		{"NUL-cut name keeps its original for the header check", []rawEntry{{name: "a.txt\x00.exe", data: "n"}}, []string{"a.txt=n"}},
	}
	dir := t.TempDir()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jar := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+".jar")
			writeFile(t, jar, rawZip("", c.entries, ""))
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
	writeFile(t, jar, rawZip(strings.Repeat("#", 77), []rawEntry{{name: "a.txt", data: "abc"}, {name: "b.txt", data: "de"}}, ""))
	if got := readAllEntries(t, jar, true); strings.Join(got, "|") != "a.txt=abc|b.txt=de" {
		t.Fatalf("%q", got)
	}
	if _, err := openZip(filepath.Join(t.TempDir(), "missing.jar")); !os.IsNotExist(err) {
		t.Fatalf("missing jar: %v", err)
	}
}

// A deflated entry whose compressed bytes stop before compress_size is
// EOFError in Python (ZipExtFile._read2 reads nothing while bytes are
// still owed), for read() and copyfileobj's bounded reads alike: the same
// errTruncatedEntry as a short stored entry. The file ends inside the
// entry's data; the zipInfo is built by hand.
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
		writeFile(t, path, append(append([]byte{}, hdr.Bytes()...), comp.Bytes()[:cut]...))
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := f.Stat()
		z := &zipFile{f: f, size: st.Size()}
		x := &zipInfo{name: "a.txt", origName: "a.txt", size: uint64(len(data)), crc: crc32.ChecksumIEEE(data),
			method: zipDeflated, compressSize: uint64(comp.Len())}
		for _, whole := range []bool{true, false} {
			if _, err := z.readEntry(x, whole); !errors.Is(err, errTruncatedEntry) {
				t.Errorf("cut %d whole=%v: %v, want errTruncatedEntry", cut, whole, err)
			}
		}
		f.Close()
	}
}
