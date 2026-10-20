package zipfile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The central-directory corners Python's zipfile decides (see zipdir.go).
func TestCentralDirFollowsPythonZipfile(t *testing.T) {
	ok := testkit.RawEntry{Name: "doc/a.html", Data: "a"}
	cases := []struct {
		name    string
		data    []byte
		want    []string // nil = unreadable
		wantErr error
	}{
		{"cp437 name", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/\x81ber.html", Data: "x"}}, ""), []string{"doc/über.html"}, nil},
		{"NUL ends the name", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.html\x00.exe", Data: "x"}}, ""), []string{"doc/a.html"}, nil},
		{"UTF-8 flag on a UTF-8 name", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/ñ.html", Flags: 0x800, Data: "x"}}, ""), []string{"doc/ñ.html"}, nil},
		{"UTF-8 flag on bytes that are not UTF-8", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/\xff.html", Flags: 0x800}}, ""), nil, nil},
		{"Unicode path extra field", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("doc/x.html", "doc/ñ.html")}}, ""), []string{"doc/ñ.html"}, nil},
		{"Unicode path with another CRC", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("other", "doc/ñ.html")}}, ""), []string{"doc/x.html"}, nil},
		{"empty Unicode path", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("doc/x.html", "")}}, ""), []string{"doc/x.html"}, nil},
		{"Unicode path not UTF-8", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("doc/x.html", "\xff")}}, ""), nil, nil},
		{"zip64 size", testkit.RawZip("", []testkit.RawEntry{{Name: "big.txt", Data: "12345", Size64: true, Extra: testkit.Zip64Size(5)}}, ""), []string{"big.txt"}, nil},
		{"zip64 extra too short", testkit.RawZip("", []testkit.RawEntry{{Name: "big.txt", Size64: true, Extra: testkit.ExtraField(1, []byte{1, 2})}}, ""), nil, nil},
		{"extra field longer than the record", testkit.RawZip("", []testkit.RawEntry{{Name: "a.txt", Extra: []byte{0x99, 0x99, 0x10, 0}}}, ""), nil, nil},
		{"prepended data", testkit.RawZip(strings.Repeat("#", 100), []testkit.RawEntry{ok}, ""), []string{"doc/a.html"}, nil},
		{"archive comment", testkit.RawZip("", []testkit.RawEntry{ok}, "a comment"), []string{"doc/a.html"}, nil},
		{"extract version above 6.3", testkit.RawZip("", []testkit.RawEntry{{Name: "a.txt", Extract: 64}}, ""), nil, errUnsupportedZip},
		{"zip64 end record", testkit.Zip64Zip("", []testkit.RawEntry{ok, {Name: "b.txt", Data: "b"}}, 1, 0), []string{"doc/a.html", "b.txt"}, nil},
		{"zip64 end record after prepended data", testkit.Zip64Zip(strings.Repeat("#", 50), []testkit.RawEntry{ok}, 1, 0), []string{"doc/a.html"}, nil},
		{"zip64 on several disks", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 2, 0), nil, nil},
		{"zip64 record offset past the record", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 1, 1000), nil, nil},
		{"zip64 record offset short of the record", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 1, -1), nil, nil},
		{"directory walked by size, not by count", testkit.WithEntryCount(testkit.RawZip("", []testkit.RawEntry{ok, {Name: "b.txt", Data: "b"},
			{Name: "c/", Data: ""}, {Name: "d.txt", Data: "d"}}, ""), 1), []string{"doc/a.html", "b.txt", "c/", "d.txt"}, nil},
		{"not a zip", []byte("plain text, no zip at all"), nil, nil},
		{"empty file", []byte{}, nil, nil},
		{"truncated", testkit.RawZip("", []testkit.RawEntry{ok}, "")[:40], nil, nil},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jar := filepath.Join(dir, fmt.Sprintf("%d.jar", i))
			testkit.WriteFile(t, jar, c.data)
			infos, err := ReadCentralDir(jar)
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
				names = append(names, e.Name)
			}
			if !reflect.DeepEqual(names, c.want) {
				t.Fatalf("names %q, want %q", names, c.want)
			}
		})
	}
	if _, err := ReadCentralDir(dir); err == nil {
		t.Fatal("a directory read as a zip")
	}
	if _, err := ReadCentralDir(filepath.Join(dir, "missing.jar")); !os.IsNotExist(err) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestZip64SizeIsReported(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "big.jar")
	testkit.WriteFile(t, jar, testkit.RawZip("", []testkit.RawEntry{{Name: "big.txt", Data: "12345", Size64: true, Extra: testkit.Zip64Size(5)}}, ""))
	if infos, err := ReadCentralDir(jar); err != nil || infos[0].Size != 5 {
		t.Fatalf("entries %+v, %v", infos, err)
	}
}
