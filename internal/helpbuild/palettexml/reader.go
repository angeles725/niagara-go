package palettexml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
)

// reader holds the walk of one document: the module prefixes declared so far,
// the open <p> elements (open[i] is the node an element at depth i+2 made, nil
// below an entry's properties) and the entry they sit in.
type reader struct {
	prefixes map[string]string
	root     *Node
	open     []*Node
	entry    *Node // the entry being read, nil while in folders
	under    int   // how many open <p> are below entry
	entries  []*Node
}

// typeOf resolves the t attribute of a <p> ("" when absent), declaring its m first.
func (r *reader) typeOf(a map[string]string) (string, error) {
	if m, ok := a["m"]; ok {
		prefix, module, found := strings.Cut(m, "=")
		if !found || prefix == "" || module == "" {
			return "", fmt.Errorf("module declaration m=%q is not prefix=module", m)
		}
		r.prefixes[prefix] = module
	}
	t := a["t"]
	if t == "" {
		return "", nil
	}
	prefix, name, found := strings.Cut(t, ":")
	module, known := r.prefixes[prefix]
	if !found || !known {
		return "", fmt.Errorf("type t=%q names an undeclared module prefix", t)
	}
	return module + ":" + name, nil
}

// start reads one element at depth (1 is the bajaObjectGraph).
func (r *reader) start(depth int, se xml.StartElement) error {
	if depth == 1 {
		if se.Name.Local != "bajaObjectGraph" {
			return fmt.Errorf("root element is <%s>, not <bajaObjectGraph>", se.Name.Local)
		}
		return nil
	}
	if se.Name.Local != "p" {
		return nil
	}
	a := make(map[string]string, len(se.Attr))
	for _, at := range se.Attr {
		a[at.Name.Local] = at.Value
	}
	typ, err := r.typeOf(a)
	if err != nil {
		return err
	}
	n := &Node{Name: bog.UnescapeName(a["n"]), Type: typ}
	switch {
	case depth == 2 && r.root != nil:
		return errors.New("the bajaObjectGraph holds more than one palette <p>")
	case depth == 2:
		n.Folder = true
		r.root = n
	case r.entry != nil:
		r.under++
		if r.under == 1 {
			r.entry.Properties = append(r.entry.Properties, Property{n.Name, typ, cut(a["v"])})
		}
		if typ != "" {
			r.entry.Contains = append(r.entry.Contains, typ)
		}
		n = nil
	default:
		parent := r.open[len(r.open)-1]
		n.Folder = isFolder(typ)
		parent.Children = append(parent.Children, n)
		if !n.Folder {
			r.entry = n
			r.entries = append(r.entries, n)
		}
	}
	r.open = append(r.open, n)
	return nil
}

// end closes the innermost <p>.
func (r *reader) end() {
	r.open = r.open[:len(r.open)-1]
	switch {
	case r.under > 0:
		r.under--
	case r.entry != nil:
		r.entry = nil
	}
}

// cut keeps the first MaxValue characters of v, marking a cut with "…".
func cut(v string) string {
	if rs := []rune(v); len(rs) > MaxValue {
		return string(rs[:MaxValue]) + "…"
	}
	return v
}
