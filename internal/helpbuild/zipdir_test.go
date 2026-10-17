package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The central-directory corners Python's zipfile decides (see zipdir.go).
func TestCentralDirFollowsPythonZipfile(t *testing.T) {
	ok := rawEntry{name: "doc/a.html", data: "a"}
	cases := []struct {
		name    string
		data    []byte
		want    []string // nil = unreadable
		wantErr error
	}{
		{"cp437 name", rawZip("", []rawEntry{{name: "doc/\x81ber.html", data: "x"}}, ""), []string{"doc/über.html"}, nil},
		{"NUL ends the name", rawZip("", []rawEntry{{name: "doc/a.html\x00.exe", data: "x"}}, ""), []string{"doc/a.html"}, nil},
		{"UTF-8 flag on a UTF-8 name", rawZip("", []rawEntry{{name: "doc/ñ.html", flags: 0x800, data: "x"}}, ""), []string{"doc/ñ.html"}, nil},
		{"UTF-8 flag on bytes that are not UTF-8", rawZip("", []rawEntry{{name: "doc/\xff.html", flags: 0x800}}, ""), nil, nil},
		{"Unicode path extra field", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("doc/x.html", "doc/ñ.html")}}, ""), []string{"doc/ñ.html"}, nil},
		{"Unicode path with another CRC", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("other", "doc/ñ.html")}}, ""), []string{"doc/x.html"}, nil},
		{"empty Unicode path", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("doc/x.html", "")}}, ""), []string{"doc/x.html"}, nil},
		{"Unicode path not UTF-8", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("doc/x.html", "\xff")}}, ""), nil, nil},
		{"zip64 size", rawZip("", []rawEntry{{name: "big.txt", data: "12345", size64: true, extra: zip64Size(5)}}, ""), []string{"big.txt"}, nil},
		{"zip64 extra too short", rawZip("", []rawEntry{{name: "big.txt", size64: true, extra: extraField(1, []byte{1, 2})}}, ""), nil, nil},
		{"extra field longer than the record", rawZip("", []rawEntry{{name: "a.txt", extra: []byte{0x99, 0x99, 0x10, 0}}}, ""), nil, nil},
		{"prepended data", rawZip(strings.Repeat("#", 100), []rawEntry{ok}, ""), []string{"doc/a.html"}, nil},
		{"archive comment", rawZip("", []rawEntry{ok}, "a comment"), []string{"doc/a.html"}, nil},
		{"extract version above 6.3", rawZip("", []rawEntry{{name: "a.txt", extract: 64}}, ""), nil, ErrUnsupportedZip},
		{"not a zip", []byte("plain text, no zip at all"), nil, nil},
		{"empty file", []byte{}, nil, nil},
		{"truncated", rawZip("", []rawEntry{ok}, "")[:40], nil, nil},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jar := filepath.Join(dir, fmt.Sprintf("%d.jar", i))
			writeFile(t, jar, c.data)
			infos, err := readCentralDir(jar)
			if c.want == nil {
				if err == nil {
					t.Fatalf("entries %+v, want unreadable", infos)
				}
				if c.wantErr != nil && err != c.wantErr {
					t.Fatalf("err %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unreadable: %v", err)
			}
			var names []string
			for _, e := range infos {
				names = append(names, e.name)
			}
			if !reflect.DeepEqual(names, c.want) {
				t.Fatalf("names %q, want %q", names, c.want)
			}
		})
	}
	if _, err := readCentralDir(dir); err == nil {
		t.Fatal("a directory read as a zip")
	}
	if _, err := readCentralDir(filepath.Join(dir, "missing.jar")); !os.IsNotExist(err) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestZip64SizeIsReported(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "big.jar")
	writeFile(t, jar, rawZip("", []rawEntry{{name: "big.txt", data: "12345", size64: true, extra: zip64Size(5)}}, ""))
	if infos, err := readCentralDir(jar); err != nil || infos[0].size != 5 {
		t.Fatalf("entries %+v, %v", infos, err)
	}
}
