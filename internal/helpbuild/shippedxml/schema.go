package shippedxml

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Schema is what nh keeps of one .xsd or .dtd: never its text, only what
// names it. For an XML Schema, Root is the root element's local name (schema),
// TargetNamespace its targetNamespace and Elements the top-level element
// declarations; for a DTD, Root is the first element it declares (the usual
// document element) and Elements the <!ELEMENT declarations outside comments.
type Schema struct {
	Path, Kind, Root, TargetNamespace string
	Elements                          int
}

// IsSchema reports whether a jar member is an .xsd or .dtd file.
func IsSchema(member string) bool {
	return strings.HasSuffix(member, ".xsd") || strings.HasSuffix(member, ".dtd")
}

var (
	dtdComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	dtdElement = regexp.MustCompile(`<!ELEMENT\s+([^\s>]+)`)
)

// ParseSchema reads the schema at jar member path. A DTD is not XML and is
// only scanned; an XML Schema that is not well-formed is an error.
func ParseSchema(path string, data []byte) (Schema, error) {
	if strings.HasSuffix(path, ".dtd") {
		s := Schema{Path: path, Kind: "dtd"}
		decls := dtdElement.FindAllSubmatch(dtdComment.ReplaceAll(data, nil), -1)
		if len(decls) > 0 {
			s.Root = string(decls[0][1])
		}
		s.Elements = len(decls)
		return s, nil
	}
	s := Schema{Path: path, Kind: "xsd"}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = charsetReader
	for depth := 0; ; {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Schema{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				s.Root = t.Name.Local
				for _, a := range t.Attr {
					if a.Name.Local == "targetNamespace" {
						s.TargetNamespace = a.Value
					}
				}
			} else if depth == 2 && t.Name.Local == "element" {
				s.Elements++
			}
		case xml.EndElement:
			depth--
		}
	}
	if s.Root == "" {
		return Schema{}, errors.New("no root element")
	}
	return s, nil
}

// charsetReader decodes the single-byte encodings a schema may declare
// besides UTF-8 (saml-rt's assertion schema declares US-ASCII): ASCII is
// UTF-8 already, ISO-8859-1 maps each byte to the rune of the same number.
func charsetReader(label string, in io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "us-ascii", "ascii":
		return in, nil
	case "iso-8859-1", "latin1", "latin-1":
		var b strings.Builder
		r := bufio.NewReader(in)
		for {
			c, err := r.ReadByte()
			if errors.Is(err, io.EOF) {
				return strings.NewReader(b.String()), nil
			}
			if err != nil {
				return nil, err
			}
			b.WriteRune(rune(c))
		}
	}
	return nil, fmt.Errorf("unsupported encoding %q", label)
}
