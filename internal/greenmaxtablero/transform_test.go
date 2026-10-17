package greenmaxtablero

import (
	"math"
	"strings"
	"testing"
)

func TestCheckWellFormed(t *testing.T) {
	for doc, ok := range map[string]bool{
		`<?xml version="1.0"?><!-- c --><a><b/></a>`: true,
		"<a/>\n<!-- trailing -->\n":                  true,
		"<a/><b/>":                                   false,
		"<a></a>junk":                                false,
		"<a><b></a>":                                 false,
		"<a>&nbsp;</a>":                              false,
		"<!-- only a comment -->":                    false,
		"<a>":                                        false,
	} {
		if err := checkWellFormed(doc); (err == nil) != ok {
			t.Errorf("checkWellFormed(%q) = %v, want ok=%v", doc, err, ok)
		}
	}
}

func TestBadCoordinateIsARefusal(t *testing.T) {
	px := `<CanvasPane><Picture layout="1.2.3,4.0,60.0,30.0"><ValueBinding ord="slot:Relay$5b1$5d$2eBO"><IBooleanToSimple/></ValueBinding></Picture></CanvasPane>`
	_, _, _, err := transform(px, "GM1", false)
	if err == nil || !strings.Contains(err.Error(), "could not convert string to float: '1.2.3'") {
		t.Errorf("err = %v", err)
	}
}

func TestStatsRepr(t *testing.T) {
	s := stats{circuits: 2, missing: []int{}, bumped: &[2]float64{600, 702}}
	if got := s.String(); !strings.HasSuffix(got, "view_bumped=(600.0, 702)") || !strings.Contains(got, "missing=[]") {
		t.Errorf("stats = %q", got)
	}
}

// minidom (expat, namespace mode) is the reference: every document below is parsed by
// xml.dom.minidom.parseString with the verdict noted (Python 3.15, expat error in the comment).
func TestCheckWellFormedMatchesMinidom(t *testing.T) {
	for _, tc := range []struct {
		doc string
		ok  bool
	}{
		{`<a x="1" x="2"/>`, false},                               // duplicate attribute
		{`<a xmlns:p="u" xmlns:q="u" p:x="1" q:x="2"/>`, false},   // duplicate attribute (same expanded name)
		{`<a x="1" p:x="2" xmlns:p="u"/>`, true},                  // different expanded names
		{"<a/>\u00a0", false},                                     // not well-formed (invalid token)
		{"<a/>\n\u00a0\n", false},                                 // not well-formed (invalid token)
		{"<a/>\u0085", false},                                     // not well-formed (invalid token)
		{"<a/>\u2028", false},                                     // not well-formed (invalid token)
		{"\u00a0<a/>", false},                                     // not well-formed (invalid token)
		{"<a/>\t\n\r ", true},                                     // XML whitespace
		{"\ufeff<a/>", true},                                      // a leading byte order mark
		{"\ufeff<?xml version=\"1.0\"?><a/>", true},               // BOM then the declaration
		{`<a>&#xD800;</a>`, false},                                // reference to invalid character number
		{`<a x="&#xD800;"/>`, false},                              // reference to invalid character number
		{`<a>&#x9;&#xA;&#xD;&#x20;</a>`, true},                    // valid character references
		{`<a><!-- &#xD800; --></a>`, true},                        // a comment is not parsed for references
		{`<a><![CDATA[&#xD800;]]></a>`, true},                     // nor is a CDATA section
		{`<?xml version="1.0"?><?xml version="1.0"?><a/>`, false}, // XML declaration not at start
		{`  <?xml version="1.0"?><a/>`, false},                    // XML declaration not at start
		{`<?XML x?><a/>`, false},                                  // reserved processing-instruction target
		{`<?xml-foo?><a/>`, true},                                 // an ordinary target
		{`<p:b/>`, false},                                         // unbound prefix
		{`<a p:x="1"/>`, false},                                   // unbound prefix (attribute)
		{`<xmlns:a/>`, false},                                     // unbound prefix
		{`<a xmlns:p=""/>`, false},                                // must not undeclare prefix
		{`<a xmlns:p="u"><p:b/></a>`, true},                       // a bound prefix
		{`<a xml:lang="en"/>`, true},                              // xml is always bound
		{`<!DOCTYPE a><a/>`, true},                                // a document type declaration
		{`<!-- c --><!DOCTYPE a><a/>`, true},                      // after a comment
		{`<a/><!DOCTYPE a>`, false},                               // junk after document element
		{`<a><!DOCTYPE a></a>`, false},                            // not well-formed (invalid token)
		{`<!DOCTYPE a><!DOCTYPE a><a/>`, false},                   // syntax error
		{`<!FOO><a/>`, false},                                     // not well-formed (invalid token)
	} {
		if err := checkWellFormed(tc.doc); (err == nil) != tc.ok {
			t.Errorf("checkWellFormed(%q) = %v, want ok=%v", tc.doc, err, tc.ok)
		}
	}
}

// pyFloat is float(): a digit string too large for a float64 is inf, not an error.
func TestPyFloatOverflowIsInf(t *testing.T) {
	big := strings.Repeat("9", 400)
	for _, s := range []string{big, big + ".5", "1." + strings.Repeat("9", 400)} {
		v, err := pyFloat(s)
		want := math.Inf(1)
		if s[0] == '1' {
			want = 2 // 1.999... rounds to 2.0, as in Python
		}
		if err != nil || v != want {
			t.Errorf("pyFloat(%.12q...) = %v, %v; want %v", s, v, err, want)
		}
	}
}

// pill is a one-circuit Px with the given viewSize and pill position.
func pill(viewSize, xy string) string {
	return `<CanvasPane viewSize="` + viewSize + `"><Picture layout="` + xy + `,60.0,30.0">` +
		`<ValueBinding ord="slot:Relay$5b1$5d$2eBO"><IBooleanToSimple/></ValueBinding></Picture></CanvasPane>`
}

// What the Python tool does with coordinates past float64 or int64 (outputs checked against
// greenmax-tablero-gen.py --stdout).
func TestHugeCoordinatesFollowPython(t *testing.T) {
	big := strings.Repeat("9", 400)
	px, _, st, err := transform(pill(big+".0,100.0", "10.0,200.0"), "GM1", false)
	if err != nil || !strings.Contains(px, `viewSize="inf,272.0"`) || st.bumped == nil || *st.bumped != [2]float64{100, 272} {
		t.Errorf("width overflow: err %v, bumped %v, px %.80q", err, st.bumped, px)
	}
	px, _, st, err = transform(pill("900.0,"+big, "10.0,200.0"), "GM1", false)
	if err != nil || st.bumped != nil || !strings.Contains(px, `viewSize="900.0,`+big+`"`) {
		t.Errorf("height overflow: err %v, bumped %v", err, st.bumped)
	}
	_, _, _, err = transform(pill("900.0,100.0", big+",200.0"), "GM1", false)
	if err == nil || err.Error() != "greenmax-tablero-gen: cannot convert float infinity to integer" {
		t.Errorf("x overflow: err %v", err)
	}
	px, _, _, err = transform(pill("900.0,100.0", "1"+strings.Repeat("0", 30)+".5,200.0"), "GM1", false)
	if err != nil || !strings.Contains(px, `<Picture layout="1000000000000000019884624838656.0,230.0,60.0,22.0"`) {
		t.Errorf("x past int64: err %v, px %q", err, px)
	}
}
