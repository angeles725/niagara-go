package nlicense

import (
	"strings"
	"testing"
)

// TestParseXMLDepthLimit mirrors libxml2's default 256-level element depth
// (lxml without huge_tree); texts captured from lxml 6.1.3 / libxml2 2.14.6.
func TestParseXMLDepthLimit(t *testing.T) {
	doc := func(open string, n int) []byte {
		return []byte("<license>" + strings.Repeat(open, n-1) + "</license>")
	}
	if _, err := parseXML([]byte("<license>" + strings.Repeat("<a>", 255) + strings.Repeat("</a>", 255) + "</license>")); err != nil {
		t.Errorf("depth 256: %v", err)
	}
	for _, tc := range []struct {
		name string
		xml  []byte
		want string
	}{
		{"open-257", doc("<a>", 257), "line 1, column 777"},
		{"open-10M", doc("<a>", 10_000_000), "line 1, column 777"},
		{"empty-257", []byte("<license>" + strings.Repeat("<a>", 255) + "<b/>"), "line 1, column 777"},
		{"attrs-257", []byte("<license>" + strings.Repeat("<a>", 255) + `<bé  y="2"  >`), "line 1, column 787"},
		{"multiline", []byte("<license>\n" + strings.Repeat("<a x=\"1\">\n", 400)), "line 257, column 9"},
	} {
		_, err := parseXML(tc.xml)
		want := "Excessive depth in document: 256, use XML_PARSE_HUGE option, " + tc.want
		if err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %q", tc.name, err, want)
		}
	}
}
