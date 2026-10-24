package pyxml

import "testing"

// TestTextModeIgnoresDeclaredEncoding: ET.fromstring(str) parses the string as UTF-8 whatever
// the declaration names (CPython 3.15, expat 2.9.0), and Iter/FindAll walk the tree like ET.
func TestTextModeIgnoresDeclaredEncoding(t *testing.T) {
	for _, enc := range []string{"ISO-8859-1", "bogus", "utf-16", "us-ascii"} {
		doc := `<?xml version="1.0" encoding="` + enc + `"?><a><p n="é€"/><b><p/></b></a>`
		root, err := Parse([]byte(doc), Mode{ETree: true, Text: true})
		if err != nil {
			t.Fatalf("%s: %v", enc, err)
		}
		var ps []*Element
		root.Iter("p", func(e *Element) { ps = append(ps, e) })
		if v, _ := ps[0].Get("n"); len(ps) != 2 || v != "é€" {
			t.Errorf("%s: got %d <p>, n=%q", enc, len(ps), v)
		}
		if got := len(root.FindAll("p")); got != 1 {
			t.Errorf("%s: FindAll(p) = %d direct children, want 1", enc, got)
		}
	}
}
