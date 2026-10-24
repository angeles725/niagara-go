package audits

import (
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyxml"
)

// TestFromStringMatchesElementTree: the attributes of every <p> (root.iter("p"))
// or a parse error, as Python's xml.etree.ElementTree.fromstring gives them
// for the same text (expected values captured from Python 3.15).
func TestFromStringMatchesElementTree(t *testing.T) {
	cases := []struct {
		name, text string
		want       []map[string]string // nil: ParseError
	}{
		{"nested p in document order", `<a><p n="1" t="x"/><b><p m="m"/></b><p/></a>`,
			[]map[string]string{{"n": "1", "t": "x"}, {"m": "m"}, {}}},
		{"declaration after a comment", "<!-- c -->\n<?xml version=\"1.0\"?><a/>", nil},
		{"declaration after a space", ` <?xml version="1.0"?><a/>`, nil},
		{"byte order mark", "\ufeff<?xml version=\"1.0\" encoding=\"UTF-8\"?><a><p n=\"x\"/></a>",
			[]map[string]string{{"n": "x"}}},
		{"second root", `<a/><b/>`, nil},
		{"text after the root", `<a/>x`, nil},
		{"text before the root", `x<a/>`, nil},
		{"mismatched tag", `<a><b></a>`, nil},
		{"unbound prefix", `<x:a/>`, nil},
		{"unbound attribute prefix", `<a><p y:n="1"/></a>`, nil},
		{"prefix declared on the same element", `<a><p xmlns:x="u" x:n="1"/></a>`,
			[]map[string]string{{"{u}n": "1"}}},
		{"element prefix declared on itself", `<a><x:p xmlns:x="u" n="1"/></a>`, []map[string]string{}},
		{"prefix out of scope in a sibling", `<a><b xmlns:x="u"/><p x:n="1"/></a>`, nil},
		{"predeclared xml prefix", `<a><p xml:lang="en"/></a>`,
			[]map[string]string{{"{http://www.w3.org/XML/1998/namespace}lang": "en"}}},
		{"prefixed tag and attribute", `<a xmlns:x="u"><x:p n="1"/><p x:n="2"/></a>`,
			[]map[string]string{{"{u}n": "2"}}},
		{"default namespace", `<a xmlns="u"><p n="1"/></a>`, []map[string]string{}},
		{"duplicate attribute", `<a n="1" n="2"/>`, nil},
		{"entities", `<a><p n="&amp;&#65;"/></a>`, []map[string]string{{"n": "&A"}}},
		{"undefined entity", `<a>&nbsp;</a>`, nil},
		{"empty", ``, nil},
		{"comment after the root", "<a><p n=\"z\"/></a><!-- ok -->\n", []map[string]string{{"n": "z"}}},
		{"unclosed", `<a><p n="1"/>`, nil},
		{"doctype", `<!DOCTYPE a><a><p n="d"/></a>`, []map[string]string{{"n": "d"}}},
		// fromstring(str) parses the text as it is and ignores the declared encoding.
		{"declared latin-1", `<?xml version="1.0" encoding="ISO-8859-1"?><a><p n="é€"/></a>`,
			[]map[string]string{{"n": "é€"}}},
		{"declared unknown encoding", `<?xml version="1.0" encoding="bogus"?><a><p n="é"/></a>`,
			[]map[string]string{{"n": "é"}}},
		{"declared latin-1 past a read buffer", `<?xml version="1.0" encoding="latin1"?><a><p n="` +
			strings.Repeat("é", 5000) + `"/></a>`, []map[string]string{{"n": strings.Repeat("é", 5000)}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, err := fromString(c.text)
			if c.want == nil {
				if err == nil {
					t.Fatalf("parsed, want a ParseError")
				}
				return
			}
			if err != nil {
				t.Fatalf("error %v", err)
			}
			got := []map[string]string{}
			root.Iter("p", func(e *pyxml.Element) {
				m := map[string]string{}
				for _, a := range e.Attrs {
					m[a.Name] = a.Value
				}
				got = append(got, m)
			})
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestFromStringErrorText: a declaration or a DOCTYPE after the root is ElementTree's "junk after
// document element" at its start (#210; CPython 3.15, expat 2.9.0).
func TestFromStringErrorText(t *testing.T) {
	cases := []struct{ text, want string }{
		{`<r/><?xml version='1.0'?>`, "junk after document element: line 1, column 4"},
		{"<r/>\n<?xml version='1.0'?>", "junk after document element: line 2, column 0"},
		{`<?xml version='1.0'?><r/><?xml version='1.0'?>`, "junk after document element: line 1, column 25"},
		{`<r/><!DOCTYPE r>`, "junk after document element: line 1, column 4"},
		{`<a>&nbsp;</a>`, "undefined entity: line 1, column 3"},
	}
	for _, c := range cases {
		if _, err := fromString(c.text); err == nil || err.Error() != c.want {
			t.Errorf("fromString(%q) error %v, want %q", c.text, err, c.want)
		}
	}
}
