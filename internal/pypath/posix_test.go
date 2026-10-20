package pypath

import (
	"encoding/json"
	"os"
	"testing"
)

// ntJoinCases: ntpath.join(*parts) as Windows CPython 3.13 returns it (the same table
// Linux CPython's pure-Python ntpath gives).
var ntJoinCases = []struct {
	parts []string
	want  string
}{
	{[]string{""}, ""},
	{[]string{"", "", ""}, ""},
	{[]string{"a"}, "a"},
	{[]string{"/a"}, "/a"},
	{[]string{"\\a"}, "\\a"},
	{[]string{"a:"}, "a:"},
	{[]string{"a:", "\\b"}, "a:\\b"},
	{[]string{"a", "\\b"}, "\\b"},
	{[]string{"a", "b", "c"}, "a\\b\\c"},
	{[]string{"a\\", "b", "c"}, "a\\b\\c"},
	{[]string{"a", "b\\", "c"}, "a\\b\\c"},
	{[]string{"a", "b", "\\c"}, "\\c"},
	{[]string{"d:\\", "\\pleep"}, "d:\\pleep"},
	{[]string{"d:\\", "a", "b"}, "d:\\a\\b"},
	{[]string{"", "a"}, "a"},
	{[]string{"a", ""}, "a\\"},
	{[]string{"a/", ""}, "a/"},
	{[]string{"a/b", "x/y"}, "a/b\\x/y"},
	{[]string{"/a/b", "x/y"}, "/a/b\\x/y"},
	{[]string{"/a/b/", "x/y"}, "/a/b/x/y"},
	{[]string{"c:", "x/y"}, "c:x/y"},
	{[]string{"c:a/b", "x/y"}, "c:a/b\\x/y"},
	{[]string{"c:/", "x/y"}, "c:/x/y"},
	{[]string{"c:/a/b", "x/y"}, "c:/a/b\\x/y"},
	{[]string{"//computer/share", "x/y"}, "//computer/share\\x/y"},
	{[]string{"//computer/share/", "x/y"}, "//computer/share/x/y"},
	{[]string{"//computer/share/a/b", "x/y"}, "//computer/share/a/b\\x/y"},
	{[]string{"a/b", "/x/y"}, "/x/y"},
	{[]string{"c:", "/x/y"}, "c:/x/y"},
	{[]string{"c:a/b", "/x/y"}, "c:/x/y"},
	{[]string{"//computer/share", "/x/y"}, "//computer/share/x/y"},
	{[]string{"c:", "C:x/y"}, "C:x/y"},
	{[]string{"c:a/b", "C:x/y"}, "C:a/b\\x/y"},
	{[]string{"c:/", "C:x/y"}, "C:/x/y"},
	{[]string{"a/b", "D:x/y"}, "D:x/y"},
	{[]string{"c:", "D:x/y"}, "D:x/y"},
	{[]string{"//computer/share", "D:x/y"}, "D:x/y"},
	{[]string{"//computer/share", "//computer/share/x/y"}, "//computer/share/x/y"},
	{[]string{"\\\\computer\\share", "a", "b"}, "\\\\computer\\share\\a\\b"},
	{[]string{"//computer", "x"}, "//computer\\x"},
	{[]string{"//computer/", "x"}, "//computer/x"},
	{[]string{"\\\\?\\UNC\\srv\\shr", "x"}, "\\\\?\\UNC\\srv\\shr\\x"},
	{[]string{"\\\\?\\c:", "x"}, "\\\\?\\c:x"},
	{[]string{"\\\\.\\dev", "x"}, "\\\\.\\dev\\x"},
	{[]string{"shared", "px", "m", "i.png"}, "shared\\px\\m\\i.png"},
	{[]string{"./org/", "m", "*", "extracted", "i.png"}, "./org/m\\*\\extracted\\i.png"},
	{[]string{"org//", "m"}, "org//m"},
	{[]string{"C:\\st\\shared", "Imagenes"}, "C:\\st\\shared\\Imagenes"},
	{[]string{"C:\\st\\shared", "Imagenes/a.png"}, "C:\\st\\shared\\Imagenes/a.png"},
	{[]string{"C:\\st\\shared", "/etc/x"}, "C:/etc/x"},
	{[]string{"C:\\st\\shared", "D:\\x"}, "D:\\x"},
	{[]string{"c:\\a", "C:b"}, "C:\\a\\b"},
}

// posixJoinCases: posixpath.join(*parts) as CPython returns it on any host.
var posixJoinCases = []struct {
	parts []string
	want  string
}{
	{[]string{""}, ""},
	{[]string{"", "", ""}, ""},
	{[]string{"a"}, "a"},
	{[]string{"a", "b", "c"}, "a/b/c"},
	{[]string{"a/", "b"}, "a/b"},
	{[]string{"a//", "b"}, "a//b"},
	{[]string{"", "a"}, "a"},
	{[]string{"a", ""}, "a/"},
	{[]string{"a", "", "b"}, "a/b"},
	{[]string{"a", "/b"}, "/b"},
	{[]string{"/a", "b", "/c", "d"}, "/c/d"},
	{[]string{"a", "b/", ""}, "a/b/"},
	{[]string{"a\\", "b"}, "a\\/b"},
	{[]string{"C:\\st", "x"}, "C:\\st/x"},
	{[]string{"//srv/share", "x"}, "//srv/share/x"},
}

func TestNtJoin(t *testing.T) {
	for _, c := range ntJoinCases {
		if got := NtJoin(c.parts[0], c.parts[1:]...); got != c.want {
			t.Errorf("NtJoin(%q) = %q, want %q", c.parts, got, c.want)
		}
	}
}

func TestPosixJoinTable(t *testing.T) {
	for _, c := range posixJoinCases {
		if got := PosixJoin(c.parts[0], c.parts[1:]...); got != c.want {
			t.Errorf("PosixJoin(%q) = %q, want %q", c.parts, got, c.want)
		}
	}
}

// TestPosixpathFixtures replays testdata/posixpath.json, frozen posixpath.basename,
// splitext and join output of the retired pycompat testdata/gen.py (Python 3.15).
func TestPosixpathFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/posixpath.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Paths [][3]string          `json:"paths"`
		Joins [][2]json.RawMessage `json:"joins"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) == 0 || len(f.Joins) == 0 {
		t.Fatal("empty fixture")
	}
	for _, c := range f.Paths {
		p, base, root := c[0], c[1], c[2]
		gotRoot, ext := PosixSplitext(p)
		if PosixBasename(p) != base || gotRoot != root || gotRoot+ext != p {
			t.Errorf("path %q: basename %q root %q ext %q; want %q %q", p, PosixBasename(p), gotRoot, ext, base, root)
		}
	}
	for _, c := range f.Joins {
		var parts []string
		var want string
		if json.Unmarshal(c[0], &parts) != nil || json.Unmarshal(c[1], &want) != nil {
			t.Fatalf("bad join case %s", c)
		}
		if got := PosixJoin(parts[0], parts[1:]...); got != want {
			t.Errorf("PosixJoin(%q) = %q, want %q", parts, got, want)
		}
	}
}
