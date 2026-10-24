package pyxml

import "testing"

// expat29Case pins one document's verdict on CPython 3.15 with expat 2.9.0: pyexpat
// ParserCreate().Parse(str) and ElementTree.fromstring. testdata/xmlerrors.json predates
// these shapes, so they hold here without a Python.
type expat29Case struct{ in, expat, etree string }

func checkExpat29(t *testing.T, cases []expat29Case) {
	t.Helper()
	for _, c := range cases {
		if got := xmlVerdict([]byte(c.in), Mode{}); got != c.expat {
			t.Errorf("expat %q:\n got  %q\n want %q", c.in, got, c.expat)
		}
		if got := xmlVerdict([]byte(c.in), Mode{ETree: true}); got != c.etree {
			t.Errorf("etree %q:\n got  %q\n want %q", c.in, got, c.etree)
		}
	}
}

// An XML declaration after the root element is junk there, not a misplaced declaration.
func TestExpat29DeclarationAfterRoot(t *testing.T) {
	checkExpat29(t, []expat29Case{
		{"<r/><?xml version='1.0'?>", "junk after document element: line 1, column 4", "junk after document element: line 1, column 4"},
		{"<r/>\n<?xml version='1.0'?>", "junk after document element: line 2, column 0", "junk after document element: line 2, column 0"},
		{"<r/> <?xml?>", "junk after document element: line 1, column 5", "junk after document element: line 1, column 5"},
		{"<?xml version='1.0'?><r/><?xml version='1.0'?>", "junk after document element: line 1, column 25", "junk after document element: line 1, column 25"},
		{"<r/><?xml-x?>", "", ""},
	})
}

// Outside the root any non-ASCII character after '<' starts an element (its name is checked
// as the tag is read), so after the root it is junk at the '<'.
func TestExpat29NonASCIIAfterLt(t *testing.T) {
	checkExpat29(t, []expat29Case{
		{"<r/><\uFEFF", "junk after document element: line 1, column 4", "junk after document element: line 1, column 4"},
		{"<r/><é", "junk after document element: line 1, column 4", "junk after document element: line 1, column 4"},
		{"<r/>\n<·", "junk after document element: line 2, column 0", "junk after document element: line 2, column 0"},
		{"<r/><é/>", "junk after document element: line 1, column 4", "junk after document element: line 1, column 4"},
		{"<\uFEFF/>", "not well-formed (invalid token): line 1, column 1", "not well-formed (invalid token): line 1, column 1"},
		{"<é/>", "", ""},
		{"<r/><1", "not well-formed (invalid token): line 1, column 5", "not well-formed (invalid token): line 1, column 5"},
		{"<r/><:a", "junk after document element: line 1, column 4", "not well-formed (invalid token): line 1, column 5"},
	})
}

// A <!KEYWORD takes ASCII name characters only (a ':' too without namespace processing).
func TestExpat29ASCIIKeyword(t *testing.T) {
	checkExpat29(t, []expat29Case{
		{"<!é- x --><r/>", "not well-formed (invalid token): line 1, column 2", "not well-formed (invalid token): line 1, column 2"},
		{"<!DOCTYPé r><r/>", "not well-formed (invalid token): line 1, column 8", "not well-formed (invalid token): line 1, column 8"},
		{"<!:a><r/>", "not well-formed (invalid token): line 1, column 4", "not well-formed (invalid token): line 1, column 2"},
		{"<!DOCTYPE r [<!é>]><r/>", "not well-formed (invalid token): line 1, column 15", "not well-formed (invalid token): line 1, column 15"},
		{"<!DOCTYPE r [<!:x>]><r/>", "not well-formed (invalid token): line 1, column 17", "not well-formed (invalid token): line 1, column 15"},
		{"<r/><!é>", "not well-formed (invalid token): line 1, column 6", "not well-formed (invalid token): line 1, column 6"},
	})
}
