package audits

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseETMatchesElementTree: the attributes of every <p> (root.iter("p"))
// or a parse error, as Python's xml.etree.ElementTree.fromstring gives them
// for the same text (expected values captured from Python 3.15).
func TestParseETMatchesElementTree(t *testing.T) {
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
			root, err := parseET(c.text)
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
			root.iter("p", func(e *etElement) {
				m := map[string]string{}
				for _, a := range e.attrs {
					m[a.Name.Local] = a.Value
				}
				got = append(got, m)
			})
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
