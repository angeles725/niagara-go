package shippedxml

import "testing"

const obixXSD = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema id="obix"
  targetNamespace="http://obix.org/ns/schema/1.0"
  xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <!-- Simple Types -->
  <xs:simpleType name="status"/>
  <xs:element name="obj"><xs:complexType><xs:sequence><xs:element name="inner"/></xs:sequence></xs:complexType></xs:element>
  <xs:element name="ref"/>
</xs:schema>
`

const svgDTD = `<!-- This is the DTD for SVG 1.0. -->
<!ENTITY % Boolean "(false | true)">
<!ELEMENT svg (desc|title)* >
<!ELEMENT desc (#PCDATA) >
<!-- <!ELEMENT commented (x)> -->
`

func TestParseSchema(t *testing.T) {
	for _, c := range []struct {
		path, doc string
		want      Schema
	}{
		{"com/tridium/obix/rsc/obix.xsd", obixXSD,
			Schema{Path: "com/tridium/obix/rsc/obix.xsd", Kind: "xsd", Root: "schema",
				TargetNamespace: "http://obix.org/ns/schema/1.0", Elements: 2}},
		{"org/apache/batik/dom/svg/resources/svg10.dtd", svgDTD,
			Schema{Path: "org/apache/batik/dom/svg/resources/svg10.dtd", Kind: "dtd", Root: "svg", Elements: 2}},
	} {
		got, err := ParseSchema(c.path, []byte(c.doc))
		if err != nil || got != c.want {
			t.Errorf("%s: %+v %v", c.path, got, err)
		}
	}
}

// TestParseSchemaReadsSingleByteEncodings: saml-rt's assertion schema
// declares US-ASCII; ISO-8859-1 reads too; an unknown encoding is an error.
func TestParseSchemaReadsSingleByteEncodings(t *testing.T) {
	for enc, ok := range map[string]bool{"US-ASCII": true, "ISO-8859-1": true, "EBCDIC": false} {
		high := "" // a byte above ASCII, legal in ISO-8859-1 only
		if enc == "ISO-8859-1" {
			high = "\xe9"
		}
		doc := `<?xml version="1.0" encoding="` + enc + `"?><schema targetNamespace="urn:x` + high + `"><element name="a"/></schema>`
		s, err := ParseSchema("a.xsd", []byte(doc))
		if ok != (err == nil) || (ok && s.Elements != 1) {
			t.Errorf("%s: %+v %v", enc, s, err)
		}
		if enc == "ISO-8859-1" && s.TargetNamespace != "urn:x\u00e9" {
			t.Errorf("latin-1 namespace = %q", s.TargetNamespace)
		}
	}
}

func TestParseSchemaRefusesAMalformedXSD(t *testing.T) {
	for _, doc := range []string{`<xs:schema><xs:element>`, "no xml at all"} {
		if _, err := ParseSchema("a.xsd", []byte(doc)); err == nil {
			t.Errorf("%q: no error", doc)
		}
	}
	if !IsSchema("schemas/xml.xsd") || !IsSchema("testng-1.0.dtd") || IsSchema("a.xsd.txt") {
		t.Error("IsSchema")
	}
}
