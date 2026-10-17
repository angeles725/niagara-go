package pxrender

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// element is the part of an ElementTree element the renderer reads: the tag ("{uri}local" when
// namespaced), the attributes and the child elements (comments and PIs are dropped, as ET.parse does).
type element struct {
	tag      string
	attrs    map[string]string
	children []*element
}

func (e *element) get(name string) (string, bool) {
	v, ok := e.attrs[name]
	return v, ok
}

// attr is ElementTree e.get(name) used as a truth value: "" for absent or empty.
func (e *element) attr(name string) string { return e.attrs[name] }

// find is ElementTree e.find(tag): the first direct child with that tag.
func (e *element) find(tag string) *element {
	for _, c := range e.children {
		if c.tag == tag {
			return c
		}
	}
	return nil
}

// findImage is e.find("Image[@name='<name>']").
func (e *element) findImage(name string) *element {
	for _, c := range e.children {
		if v, ok := c.get("name"); c.tag == "Image" && ok && v == name {
			return c
		}
	}
	return nil
}

// first is next(root.iter(tag)): the first element with that tag in document order, root included.
func (e *element) first(tag string) *element {
	if e.tag == tag {
		return e
	}
	for _, c := range e.children {
		if f := c.first(tag); f != nil {
			return f
		}
	}
	return nil
}

// normalizeAttrWhitespace applies XML attribute-value normalisation (what expat does and Go's
// encoding/xml does not): a literal tab, newline, CR or CRLF inside a quoted attribute value
// becomes one space. Character references such as &#10; are untouched.
func normalizeAttrWhitespace(b []byte) []byte {
	out := make([]byte, 0, len(b))
	skip := func(i int, end string) int {
		k := bytes.Index(b[i:], []byte(end))
		if k < 0 {
			return len(b)
		}
		return i + k + len(end)
	}
	for i := 0; i < len(b); {
		switch {
		case b[i] != '<':
			out = append(out, b[i])
			i++
			continue
		case bytes.HasPrefix(b[i:], []byte("<!--")):
			j := skip(i+4, "-->")
			out, i = append(out, b[i:j]...), j
			continue
		case bytes.HasPrefix(b[i:], []byte("<![CDATA[")):
			j := skip(i+9, "]]>")
			out, i = append(out, b[i:j]...), j
			continue
		case bytes.HasPrefix(b[i:], []byte("<?")):
			j := skip(i+2, "?>")
			out, i = append(out, b[i:j]...), j
			continue
		case bytes.HasPrefix(b[i:], []byte("<!")):
			j, depth := i+2, 0
			for ; j < len(b) && !(b[j] == '>' && depth == 0); j++ {
				if b[j] == '[' {
					depth++
				} else if b[j] == ']' {
					depth--
				}
			}
			j = min(j+1, len(b))
			out, i = append(out, b[i:j]...), j
			continue
		}
		var quote byte // inside a start or end tag
		for ; i < len(b); i++ {
			c := b[i]
			if quote == 0 {
				out = append(out, c)
				if c == '"' || c == '\'' {
					quote = c
				} else if c == '>' {
					i++
					break
				}
				continue
			}
			switch {
			case c == quote:
				quote = 0
				out = append(out, c)
			case c == '\r' && i+1 < len(b) && b[i+1] == '\n':
				out = append(out, ' ')
				i++
			case c == '\t' || c == '\n' || c == '\r':
				out = append(out, ' ')
			default:
				out = append(out, c)
			}
		}
	}
	return out
}

func latin1Reader(r io.Reader) io.Reader {
	data, err := io.ReadAll(r)
	if err != nil {
		return &errReader{err}
	}
	var sb strings.Builder
	for _, c := range data {
		sb.WriteRune(rune(c))
	}
	return strings.NewReader(sb.String())
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "iso-8859-1", "iso8859-1", "latin-1", "latin1", "l1":
		return latin1Reader(input), nil
	case "us-ascii", "ascii":
		return input, nil
	}
	return nil, fmt.Errorf("unsupported encoding %q", label)
}

func tagName(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return "{" + n.Space + "}" + n.Local
}

// parseXML builds the element tree of a whole document, failing where expat fails on the
// structural errors encoding/xml lets through (duplicate attributes, content after the root).
func parseXML(data []byte) (*element, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	d := xml.NewDecoder(bytes.NewReader(normalizeAttrWhitespace(data)))
	d.CharsetReader = charsetReader
	var root *element
	var stack []*element
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if root != nil && len(stack) == 0 {
				return nil, fmt.Errorf("junk after document element: <%s>", t.Name.Local)
			}
			e := &element{tag: tagName(t.Name), attrs: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns" {
					continue
				}
				k := tagName(a.Name)
				if _, dup := e.attrs[k]; dup {
					return nil, fmt.Errorf("duplicate attribute %s on <%s>", k, t.Name.Local)
				}
				e.attrs[k] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.children = append(p.children, e)
			} else {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 && len(bytes.TrimSpace(t)) > 0 {
				return nil, errors.New("text outside the document element")
			}
		}
	}
	if root == nil {
		return nil, errors.New("no element found")
	}
	return root, nil
}
