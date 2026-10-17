// Start tags, end tags and references of the expat-compatible parser (see pyxml.go).

package pyxml

import "strings"

type rawAttr struct {
	name   string
	pos    int // index of the name
	vs, ve int // value runes between the quotes
}

// startTag parses "<name attr='v'...>" or "/>" from its '<'.
func (p *xparser) startTag(s int) (int, error) {
	i, err := p.scanQName(s + 1)
	if err != nil {
		return s, err
	}
	qname := string(p.r[s+1 : i])
	var raws []rawAttr
	empty := false
	for done := false; !done; {
		sawS := false
		for !p.eof(i) && isS(p.r[i]) {
			i, sawS = i+1, true
		}
		if p.eof(i) {
			return s, p.fail(s, errUnclosed)
		}
		switch c := p.r[i]; {
		case c == '>':
			i, done = i+1, true
		case c == '/':
			if p.eof(i + 1) {
				return s, p.fail(s, errUnclosed)
			}
			if p.r[i+1] != '>' {
				return s, p.fail(i+1, errInvalid)
			}
			i, done, empty = i+2, true, true
		case sawS && p.qStart(c):
			a, next, err := p.attribute(s, i)
			if err != nil {
				return s, err
			}
			raws, i = append(raws, a), next
		default:
			return s, p.fail(i, errInvalid)
		}
	}
	return i, p.openElement(s, qname, raws, empty)
}

// attribute scans name S? = S? quoted-value starting at the name.
func (p *xparser) attribute(s, i int) (rawAttr, int, error) {
	a := rawAttr{pos: i}
	i, err := p.scanQName(i)
	if err != nil {
		return a, i, err
	}
	a.name = string(p.r[a.pos:i])
	for !p.eof(i) && isS(p.r[i]) {
		i++
	}
	if p.eof(i) {
		return a, i, p.fail(s, errUnclosed)
	}
	if p.r[i] != '=' {
		return a, i, p.fail(i, errInvalid)
	}
	for i++; !p.eof(i) && isS(p.r[i]); i++ {
	}
	if p.eof(i) {
		return a, i, p.fail(s, errUnclosed)
	}
	q := p.r[i]
	if q != '"' && q != '\'' {
		return a, i, p.fail(i, errInvalid)
	}
	a.vs = i + 1
	for i = a.vs; ; {
		if p.eof(i) {
			return a, i, p.fail(s, errUnclosed)
		}
		switch c := p.r[i]; {
		case c == q:
			a.ve = i
			return a, i + 1, nil
		case c == '<' || c == bad || !isXMLChar(c):
			return a, i, p.fail(i, errInvalid)
		case c == '&':
			e, err := p.refSyntax(i, s)
			if err != nil {
				return a, i, err
			}
			i = e
		default:
			i++
		}
	}
}

// refSyntax checks the reference at '&' a and returns the index after its ';'; a reference
// cut by the end of input is an unclosed token starting at partial.
func (p *xparser) refSyntax(a, partial int) (int, error) {
	i := a + 1
	if p.eof(i) {
		return a, p.fail(partial, errUnclosed)
	}
	if p.r[i] == '#' {
		i++
		hex := p.at(i) == 'x'
		if hex {
			i++
		}
		for first := true; ; i, first = i+1, false {
			if p.eof(i) {
				return a, p.fail(partial, errUnclosed)
			}
			if c := p.r[i]; c == ';' && !first {
				return i + 1, nil
			} else if !isDigit(c, hex) {
				return a, p.fail(i, errInvalid)
			}
		}
	}
	if !p.qStart(p.r[i]) {
		return a, p.fail(i, errInvalid)
	}
	i = p.scanNC(i)
	if p.eof(i) {
		return a, p.fail(partial, errUnclosed)
	}
	if p.r[i] != ';' {
		return a, p.fail(i, errInvalid)
	}
	return i + 1, nil
}

func isDigit(c rune, hex bool) bool {
	return '0' <= c && c <= '9' || hex && ('a' <= c && c <= 'f' || 'A' <= c && c <= 'F')
}

const (
	refOK = iota
	refUndefined
	refBadNumber
)

// refValue resolves the syntactically valid reference r[a:e] (e is after the ';').
func (p *xparser) refValue(a, e int) (string, int) {
	body := p.r[a+1 : e-1]
	if body[0] != '#' {
		name := string(body)
		if v, ok := predefined[name]; ok {
			return string(v), refOK
		}
		if v, ok := p.entities[name]; ok {
			return v, refOK
		}
		return "", refUndefined
	}
	base, digits := 10, body[1:]
	if digits[0] == 'x' {
		base, digits = 16, digits[1:]
	}
	v := 0
	for _, c := range digits {
		d := int(c - '0')
		if c > '9' {
			d = int(c|0x20-'a') + 10
		}
		if v = v*base + d; v > 0x10FFFF {
			return "", refBadNumber
		}
	}
	if !isXMLChar(rune(v)) {
		return "", refBadNumber
	}
	return string(rune(v)), refOK
}

// contentRef handles a reference in character data.
func (p *xparser) contentRef(a int) (int, error) {
	e, err := p.refSyntax(a, a)
	if err != nil {
		return a, err
	}
	switch _, kind := p.refValue(a, e); {
	case kind == refBadNumber:
		return a, p.fail(a, errBadCharRef)
	case kind == refUndefined && !p.extDTD:
		return a, p.fail(a, errUndefined)
	case kind == refUndefined && p.mode.ETree:
		return a, p.fail(a, errUndefined+" &"+string(p.r[a+1:e-1])+";")
	}
	return e, nil
}

// attrValue normalises an attribute value as expat does for CDATA attributes.
func (p *xparser) attrValue(s int, a rawAttr) (string, error) {
	var b strings.Builder
	for i := a.vs; i < a.ve; i++ {
		switch c := p.r[i]; c {
		case '&':
			e, _ := p.refSyntax(i, s)
			v, kind := p.refValue(i, e)
			switch {
			case kind == refBadNumber:
				return "", p.fail(i, errBadCharRef)
			case kind == refUndefined && !p.extDTD:
				return "", p.fail(s, errUndefined)
			}
			b.WriteString(v)
			i = e - 1
		case '\r':
			if p.at(i+1) == '\n' {
				i++
			}
			b.WriteByte(' ')
		case '\n', '\t':
			b.WriteByte(' ')
		default:
			b.WriteRune(c)
		}
	}
	return b.String(), nil
}

// openElement checks the attributes of a complete start tag, builds the element and opens it.
func (p *xparser) openElement(s int, qname string, raws []rawAttr, empty bool) error {
	el := &Element{Tag: qname}
	seen := make(map[string]bool, len(raws))
	for _, a := range raws {
		if seen[a.name] {
			return p.fail(a.pos, errDupAttr)
		}
		seen[a.name] = true
		v, err := p.attrValue(s, a)
		if err != nil {
			return err
		}
		el.Attrs = append(el.Attrs, Attr{a.name, v})
	}
	var scope map[string]string
	if len(p.stack) > 0 {
		scope = p.stack[len(p.stack)-1].ns
	}
	if p.mode.ETree {
		var err error
		if scope, err = p.namespaces(s, el, scope); err != nil {
			return err
		}
	}
	if len(p.stack) == 0 {
		p.root = el
	} else {
		parent := p.stack[len(p.stack)-1].elem
		parent.Children = append(parent.Children, el)
	}
	if !empty {
		p.stack = append(p.stack, &openElem{qname: qname, elem: el, ns: scope})
	}
	return nil
}

// namespaces applies the element's xmlns declarations and expands its names (ElementTree's
// "{uri}local"); it returns the scope for the element's content.
func (p *xparser) namespaces(s int, el *Element, parent map[string]string) (map[string]string, error) {
	scope, copied := parent, false
	kept := el.Attrs[:0:0]
	for _, a := range el.Attrs {
		prefix, isDecl := "", a.Name == "xmlns"
		if strings.HasPrefix(a.Name, "xmlns:") {
			prefix, isDecl = a.Name[6:], true
			if a.Value == "" {
				return nil, p.fail(s, errUndeclare)
			}
		}
		if !isDecl {
			kept = append(kept, a)
			continue
		}
		if !copied {
			scope, copied = make(map[string]string, len(parent)+1), true
			for k, v := range parent {
				scope[k] = v
			}
		}
		scope[prefix] = a.Value
	}
	seen := make(map[string]bool, len(kept))
	for i, a := range kept {
		if j := strings.IndexByte(a.Name, ':'); j >= 0 {
			uri, ok := lookupNS(scope, a.Name[:j])
			if !ok {
				return nil, p.fail(s, errUnbound)
			}
			kept[i].Name = "{" + uri + "}" + a.Name[j+1:]
		}
		if seen[kept[i].Name] {
			return nil, p.fail(s, errDupAttr)
		}
		seen[kept[i].Name] = true
	}
	el.Attrs = kept
	if j := strings.IndexByte(el.Tag, ':'); j >= 0 {
		uri, ok := lookupNS(scope, el.Tag[:j])
		if !ok {
			return nil, p.fail(s, errUnbound)
		}
		el.Tag = "{" + uri + "}" + el.Tag[j+1:]
	} else if uri := scope[""]; uri != "" {
		el.Tag = "{" + uri + "}" + el.Tag
	}
	return scope, nil
}

func lookupNS(scope map[string]string, prefix string) (string, bool) {
	if prefix == "xml" {
		return xmlNS, true
	}
	uri, ok := scope[prefix]
	return uri, ok && uri != ""
}

// endTag parses "</name S?>" from its '<' and closes the open element.
func (p *xparser) endTag(s int) (int, error) {
	i := s + 2
	if p.eof(i) {
		return s, p.fail(s, errUnclosed)
	}
	if !p.qStart(p.r[i]) {
		return s, p.fail(i, errInvalid)
	}
	e := p.scanName(i) // expat's end-tag scanner takes any colons
	j := e
	for !p.eof(j) && isS(p.r[j]) {
		j++
	}
	if p.eof(j) {
		return s, p.fail(s, errUnclosed)
	}
	if p.r[j] != '>' {
		return s, p.fail(j, errInvalid)
	}
	if top := p.stack[len(p.stack)-1]; string(p.r[i:e]) != top.qname {
		return s, p.fail(i, errMismatch)
	}
	p.stack = p.stack[:len(p.stack)-1]
	return j + 1, nil
}
