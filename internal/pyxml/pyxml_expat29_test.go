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
