package pxrender

import "testing"

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

func TestNTJoin(t *testing.T) {
	for _, c := range ntJoinCases {
		if got := ntJoin(c.parts[0], c.parts[1:]...); got != c.want {
			t.Errorf("ntJoin(%q) = %q, want %q", c.parts, got, c.want)
		}
	}
}
