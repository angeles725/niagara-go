package inventory

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
)

// element is the part of xml.etree.ElementTree the module.xml views use: a
// tag, its attributes and its child elements in order.
type element struct {
	tag      string
	attrs    map[string]string
	children []*element
}

// attr is Element.get(name, "").
func (e *element) attr(name string) string { return e.attrs[name] }

// find is Element.find(tag): the first direct child with that tag, or nil.
func (e *element) find(tag string) *element {
	for _, c := range e.children {
		if c.tag == tag {
			return c
		}
	}
	return nil
}

// findAll is Element.findall(tag): the direct children with that tag.
func (e *element) findAll(tag string) []*element {
	var out []*element
	for _, c := range e.children {
		if c.tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// parseXMLFile is ET.parse(path).getroot().
func parseXMLFile(path string) (*element, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	var stack []*element
	var root *element
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := &element{tag: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				e.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.children = append(p.children, e)
			} else if root == nil {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if root == nil {
		return nil, errors.New("no element found")
	}
	return root, nil
}
