package audits

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// etElement is the part of xml.etree.ElementTree palette-lexicon-agents uses:
// an element tag ("{uri}local" when namespaced, as ET names it), its
// attributes in document order and its child elements.
type etElement struct {
	tag      string
	attrs    []xml.Attr // Name.Local holds the ET attribute key
	children []*etElement
}

// get is Element.attrib.get(name).
func (e *etElement) get(name string) (string, bool) {
	for _, a := range e.attrs {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

// iter is Element.iter(tag): the element and its descendants with that tag,
// in document order.
func (e *etElement) iter(tag string, f func(*etElement)) {
	if e.tag == tag {
		f(e)
	}
	for _, c := range e.children {
		c.iter(tag, f)
	}
}

// findAll is Element.findall(tag): the direct children with that tag.
func (e *etElement) findAll(tag string) []*etElement {
	var out []*etElement
	for _, c := range e.children {
		if c.tag == tag {
			out = append(out, c)
		}
	}
	return out
}

const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

// parseET is ET.fromstring(text): the root element, or an error where expat
// reports a ParseError (mismatched or unclosed tags, an unbound prefix, a
// duplicate attribute, an undefined entity, text or a second element outside
// the root, an XML declaration after the start, no element). Namespaces resolve to "{uri}local" tags and
// attribute keys; xmlns attributes are not attributes. The document is UTF-8
// (a declared ISO-8859-1 or US-ASCII encoding is honoured, as expat does).
func parseET(text string) (*etElement, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	dec := xml.NewDecoder(strings.NewReader(text))
	dec.Strict = true
	dec.CharsetReader = charsetReader
	p := &etParser{}
	for {
		start := dec.InputOffset()
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := p.token(tok, start); err != nil {
			return nil, err
		}
	}
	if p.root == nil {
		return nil, errors.New("no element found")
	}
	if len(p.stack) > 0 {
		return nil, errors.New("unclosed token")
	}
	return p.root, nil
}

// etParser is parseET's state between raw tokens: the open elements, the
// root once it starts and whether it has closed.
type etParser struct {
	stack  []etOpen
	root   *etElement
	closed bool
}

// etOpen is one open element: the element, its raw (prefixed) name for the
// end-tag match and the namespaces it declares.
type etOpen struct {
	el  *etElement
	raw xml.Name
	ns  map[string]string // prefix -> uri declared on this element ("" = default)
}

// token applies one raw token; start is its offset in the input, which tells
// an XML declaration at the start of the entity from a later one.
func (p *etParser) token(tok xml.Token, start int64) error {
	switch t := tok.(type) {
	case xml.StartElement:
		return p.startElement(t)
	case xml.EndElement:
		if len(p.stack) == 0 || p.stack[len(p.stack)-1].raw != t.Name {
			return errors.New("mismatched tag")
		}
		p.stack = p.stack[:len(p.stack)-1]
		p.closed = len(p.stack) == 0
	case xml.CharData:
		if len(p.stack) == 0 && len(bytes.Trim(t, " \t\r\n")) > 0 {
			return errors.New("syntax error")
		}
	case xml.ProcInst:
		if t.Target == "xml" && start != 0 {
			return errors.New("XML or text declaration not at start of entity")
		}
	case xml.Directive:
		if p.root != nil {
			return errors.New("syntax error")
		}
	}
	return nil
}

// startElement opens t: its xmlns attributes declare namespaces (in scope for
// its own name and attributes, so it is pushed before they resolve), the rest
// become ET attributes, and the element joins its parent's children.
func (p *etParser) startElement(t xml.StartElement) error {
	if p.closed || (p.root != nil && len(p.stack) == 0) {
		return errors.New("junk after document element")
	}
	ns := map[string]string{}
	var plain []xml.Attr
	for _, a := range t.Attr {
		switch {
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			ns[""] = a.Value
		case a.Name.Space == "xmlns":
			ns[a.Name.Local] = a.Value
		default:
			plain = append(plain, a)
		}
	}
	p.stack = append(p.stack, etOpen{raw: t.Name, ns: ns})
	uri, ok := p.lookup(t.Name.Space)
	if !ok {
		return fmt.Errorf("unbound prefix")
	}
	el := &etElement{tag: etName(uri, t.Name.Local)}
	if err := p.setAttrs(el, plain); err != nil {
		return err
	}
	p.stack[len(p.stack)-1].el = el
	if len(p.stack) == 1 {
		p.root = el
	} else {
		parent := p.stack[len(p.stack)-2].el
		parent.children = append(parent.children, el)
	}
	return nil
}

// setAttrs gives el the non-xmlns attributes under their ET keys, in document
// order; an unbound prefix or a key seen twice (after resolution) is an error.
func (p *etParser) setAttrs(el *etElement, plain []xml.Attr) error {
	seen := map[string]bool{}
	for _, a := range plain {
		key := a.Name.Local
		if a.Name.Space != "" {
			auri, ok := p.lookup(a.Name.Space)
			if !ok {
				return fmt.Errorf("unbound prefix")
			}
			key = etName(auri, a.Name.Local)
		}
		if seen[key] {
			return errors.New("duplicate attribute")
		}
		seen[key] = true
		el.attrs = append(el.attrs, xml.Attr{Name: xml.Name{Local: key}, Value: a.Value})
	}
	return nil
}

// lookup resolves a prefix against the open elements, innermost first; "xml"
// is predeclared and the empty prefix without a default namespace is "".
func (p *etParser) lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return xmlNamespace, true
	}
	for i := len(p.stack) - 1; i >= 0; i-- {
		if uri, ok := p.stack[i].ns[prefix]; ok {
			return uri, true
		}
	}
	return "", prefix == ""
}

// etName is ET's tag or attribute key: "{uri}local", or local without a namespace.
func etName(uri, local string) string {
	if uri == "" {
		return local
	}
	return "{" + uri + "}" + local
}

// charsetReader keeps the text as it is whatever encoding the declaration
// names: ET.fromstring(str) hands expat the string already decoded and
// ignores the declared encoding (ISO-8859-1, an unknown name, ...).
func charsetReader(_ string, input io.Reader) (io.Reader, error) {
	return input, nil
}
