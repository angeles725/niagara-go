package pyhtml

import "testing"

// helpbuild's TestParityHTML5Entities checks every html5 name and its near
// misses against Python; these are the corners.
func TestIsHTML5Entity(t *testing.T) {
	for name, want := range map[string]bool{
		"amp": true, "amp;": true, "notin;": true, "notin": false, "not": true, "notit;": false,
		"NotEqualTilde;": true, "nGt;": true, "nLt;": true, "nGt": false, "alpha": false, "x": false,
	} {
		if got := IsHTML5Entity(name); got != want {
			t.Errorf("IsHTML5Entity(%q) = %v", name, got)
		}
	}
}

// html.unescape corners (values from CPython 3.15): numeric references
// out of range, surrogates, NUL, CR, the cp1252 block and the invalid code
// points; the longest named prefix; the two names Go's table lacks.
func TestUnescape(t *testing.T) {
	for in, want := range map[string]string{
		"no reference":           "no reference",
		"&amp; &lt;&gt;":         "& <>",
		"&#65;&#x42;&#X43":       "ABC",
		"&#0;&#xD800;&#x110000;": "\ufffd\ufffd\ufffd",
		"&#13;&#128;&#x9f;":      "\r\u20ac\u0178",
		"&#1;&#xb;&#xFFFE;":      "",
		"&notit; &amp":           "\u00acit; &",
		"&nGt;&nLt;&nGt":         "\u226b\u20d2\u226a\u20d2&nGt",
		"&&nGt;&#&nLt;":          "&\u226b\u20d2&#\u226a\u20d2",
		"&unknown;":              "&unknown;",
	} {
		if got := Unescape(in); got != want {
			t.Errorf("Unescape(%q) = %q, want %q", in, got, want)
		}
	}
}
