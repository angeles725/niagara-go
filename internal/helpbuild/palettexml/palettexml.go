// Package palettexml reads a Niagara palette (a module jar's *.palette: a
// bajaObjectGraph whose root <p> is the palette) into the tree nh keeps in
// indexes/palette-index.json: folders (baja:Folder, baja:UnrestrictedFolder)
// holding folders and entries, each entry with its type, its direct children
// (the properties the palette sets) and the types nested anywhere below it.
// It is a leaf: it reads bytes, not jars.
//
// Type names are resolved to "module:Type" through the m="prefix=module"
// declarations, which hold from where they appear to the end of the document,
// as the BOG decoder reads them. A document that is not well-formed, whose
// root is not a bajaObjectGraph holding a <p>, or that names a type through an
// undeclared prefix is an error, never an empty palette.
package palettexml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
)

// MaxValue is how many characters of a property value the index keeps; a
// longer value ends with "…".
const MaxValue = 120

// Palette is one palette file: its root folder (the root <p>, unnamed).
type Palette struct{ Root *Node }

// Node is a folder (Folder true, Children its folders and entries) or an
// entry (Properties its direct children, Contains the sorted types below it).
type Node struct {
	Name, Type string
	Folder     bool
	Children   []*Node
	Properties []Property
	Contains   []string
}

// Property is one direct child of an entry: its slot name, type (empty when
// the palette sets only a value) and value.
type Property struct{ Name, Type, Value string }

// isFolder reports whether a resolved type is a palette folder.
func isFolder(t string) bool { return t == "baja:Folder" || t == "baja:UnrestrictedFolder" }

// unzipped is the object graph of a zipped palette (file.xml), or data itself.
func unzipped(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return data, nil
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zipped palette: %w", err)
	}
	f, err := z.Open("file.xml")
	if err != nil {
		return nil, fmt.Errorf("zipped palette: %w", err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// Parse reads one palette document, plain or zipped.
func Parse(data []byte) (*Palette, error) {
	data, err := unzipped(data)
	if err != nil {
		return nil, err
	}
	r := reader{prefixes: map[string]string{}}
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
			if err := r.start(depth, t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == "p" && depth >= 2 {
				r.end()
			}
			depth--
		}
	}
	if r.root == nil {
		return nil, errors.New("no palette: the bajaObjectGraph holds no <p>")
	}
	for _, e := range r.entries {
		slices.Sort(e.Contains)
		e.Contains = slices.Compact(e.Contains)
	}
	return &Palette{Root: r.root}, nil
}
