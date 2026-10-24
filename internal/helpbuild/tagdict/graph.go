package tagdict

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
)

// node is one <p> of a bajaObjectGraph: its unescaped name, its type
// resolved to "module:Type" ("" when absent), its value and its children.
type node struct {
	name, typ, value string
	kids             []*node
}

// unzip is the object graph of a zipped bog (its file.xml), or data itself.
func unzip(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return data, nil
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zipped bog: %w", err)
	}
	f, err := z.Open("file.xml")
	if err != nil {
		return nil, fmt.Errorf("zipped bog: %w", err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// parseGraph reads a bog, plain or zipped, into a node holding its top
// <p>s. Types are resolved through the m="prefix=module" declarations, which
// hold from where they appear to the end of the document; an undeclared
// prefix is an error.
func parseGraph(data []byte) (*node, error) {
	data, err := unzip(data)
	if err != nil {
		return nil, err
	}
	prefixes := map[string]string{}
	top := &node{}
	open := []*node{}
	d := xml.NewDecoder(bytes.NewReader(data))
	for depth := 0; ; {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 && t.Name.Local != "bajaObjectGraph" {
				return nil, fmt.Errorf("root element is <%s>, not <bajaObjectGraph>", t.Name.Local)
			}
			if t.Name.Local != "p" || depth == 1 {
				continue
			}
			n, err := pNode(t, prefixes)
			if err != nil {
				return nil, err
			}
			parent := top
			if len(open) > 0 {
				parent = open[len(open)-1]
			}
			parent.kids = append(parent.kids, n)
			open = append(open, n)
		case xml.EndElement:
			if t.Name.Local == "p" && len(open) > 0 {
				open = open[:len(open)-1]
			}
			depth--
		}
	}
	return top, nil
}

// pNode is the node of one <p>, declaring its m first.
func pNode(se xml.StartElement, prefixes map[string]string) (*node, error) {
	a := map[string]string{}
	for _, at := range se.Attr {
		a[at.Name.Local] = at.Value
	}
	if m, ok := a["m"]; ok {
		prefix, module, found := strings.Cut(m, "=")
		if !found || prefix == "" || module == "" {
			return nil, fmt.Errorf("module declaration m=%q is not prefix=module", m)
		}
		prefixes[prefix] = module
	}
	n := &node{name: bog.UnescapeName(a["n"]), value: a["v"]}
	if t := a["t"]; t != "" {
		prefix, name, found := strings.Cut(t, ":")
		module, known := prefixes[prefix]
		if !found || !known {
			return nil, fmt.Errorf("type t=%q names an undeclared module prefix", t)
		}
		n.typ = module + ":" + name
	}
	return n, nil
}
