package nlicense

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A small XML 1.0 reader with the libxml2 behaviour the license tool relies
// on (lxml.etree.parse, resolve_entities=False, blank text kept): line ends
// normalised to \n, attribute whitespace normalised to spaces, CDATA merged
// into text, the five predefined entities and character references resolved.

type nodeKind int

const (
	elemNode nodeKind = iota
	textNode
	commentNode
	piNode
	doctypeNode
)

type attr struct{ name, value string }

type node struct {
	kind     nodeKind
	name     string // element qname, PI target, doctype text
	attrs    []attr
	children []*node
	text     string // text, comment or PI data
	line     int    // start line of an element (for libxml2 error texts)
}

// document is the parsed file: misc nodes around the root, in order.
type document struct {
	before []*node
	root   *node
	after  []*node
}

func (n *node) get(name string) (string, bool) {
	for _, a := range n.attrs {
		if a.name == name {
			return a.value, true
		}
	}
	return "", false
}

// set mirrors _Element.set: replace in place or append.
func (n *node) set(name, value string) {
	for i := range n.attrs {
		if n.attrs[i].name == name {
			n.attrs[i].value = value
			return
		}
	}
	n.attrs = append(n.attrs, attr{name, value})
}

type syntaxError struct{ msg string }

func (e syntaxError) Error() string { return e.msg }

type xmlParser struct {
	s    string
	i    int
	line int
}

func (p *xmlParser) fail(format string, a ...any) {
	line := 1 + strings.Count(p.s[:min(p.i, len(p.s))], "\n")
	panic(syntaxError{fmt.Sprintf(format+", line %d", append(a, line)...)})
}

// parseXML parses raw file bytes into a document.
func parseXML(data []byte) (doc *document, err error) {
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(syntaxError)
			if !ok {
				panic(r)
			}
			doc, err = nil, se
		}
	}()
	text, err := toUTF8(data)
	if err != nil {
		return nil, err
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	p := &xmlParser{s: text}
	doc = &document{}
	if strings.HasPrefix(p.s, "<?xml") && len(p.s) > 5 && isSpace(p.s[5]) {
		end := strings.Index(p.s, "?>")
		if end < 0 {
			p.fail("XML declaration not finished")
		}
		p.i = end + 2
	}
	for {
		p.skipSpace()
		if p.i >= len(p.s) {
			break
		}
		if !strings.HasPrefix(p.s[p.i:], "<") {
			p.fail("Extra content at the end of the document")
		}
		switch {
		case strings.HasPrefix(p.s[p.i:], "<!--"):
			n := p.comment()
			if doc.root == nil {
				doc.before = append(doc.before, n)
			} else {
				doc.after = append(doc.after, n)
			}
		case strings.HasPrefix(p.s[p.i:], "<?"):
			n := p.pi()
			if doc.root == nil {
				doc.before = append(doc.before, n)
			} else {
				doc.after = append(doc.after, n)
			}
		case strings.HasPrefix(p.s[p.i:], "<!DOCTYPE"):
			if doc.root != nil {
				p.fail("Extra content at the end of the document")
			}
			doc.before = append(doc.before, p.doctype())
		default:
			if doc.root != nil {
				p.fail("Extra content at the end of the document")
			}
			doc.root = p.element()
		}
	}
	if doc.root == nil {
		p.fail("Document is empty")
	}
	return doc, nil
}

// toUTF8 strips a UTF-8 BOM and decodes the declared single-byte encodings.
func toUTF8(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	enc := ""
	if bytes.HasPrefix(data, []byte("<?xml")) {
		if end := bytes.Index(data, []byte("?>")); end > 0 {
			decl := string(data[:end])
			if i := strings.Index(decl, "encoding"); i >= 0 {
				rest := strings.TrimLeft(decl[i+len("encoding"):], " \t\n\r=")
				if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
					if j := strings.IndexByte(rest[1:], rest[0]); j >= 0 {
						enc = strings.ToLower(rest[1 : 1+j])
					}
				}
			}
		}
	}
	switch enc {
	case "", "utf-8", "utf8":
		if !utf8.Valid(data) {
			return "", syntaxError{"Input is not proper UTF-8, indicate encoding !"}
		}
		return string(data), nil
	case "iso-8859-1", "latin1", "latin-1", "iso_8859-1":
		rs := make([]rune, len(data))
		for i, b := range data {
			rs[i] = rune(b)
		}
		return string(rs), nil
	case "us-ascii", "ascii":
		for _, b := range data {
			if b >= 0x80 {
				return "", syntaxError{"Input is not proper ASCII"}
			}
		}
		return string(data), nil
	}
	return "", syntaxError{fmt.Sprintf("Unsupported encoding: %s", enc)}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func (p *xmlParser) skipSpace() {
	for p.i < len(p.s) && isSpace(p.s[p.i]) {
		p.i++
	}
}

func isNameRune(r rune, first bool) bool {
	if r == '_' || r == ':' || unicode.IsLetter(r) {
		return true
	}
	return !first && (r == '-' || r == '.' || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || r == 0xB7)
}

func (p *xmlParser) name() string {
	start := p.i
	for p.i < len(p.s) {
		r, size := utf8.DecodeRuneInString(p.s[p.i:])
		if !isNameRune(r, p.i == start) {
			break
		}
		p.i += size
	}
	if p.i == start {
		p.fail("StartTag: invalid element name")
	}
	return p.s[start:p.i]
}

func (p *xmlParser) comment() *node {
	end := strings.Index(p.s[p.i+4:], "-->")
	if end < 0 {
		p.fail("Comment not terminated")
	}
	n := &node{kind: commentNode, text: p.s[p.i+4 : p.i+4+end]}
	p.i += 4 + end + 3
	return n
}

func (p *xmlParser) pi() *node {
	p.i += 2
	target := p.name()
	end := strings.Index(p.s[p.i:], "?>")
	if end < 0 {
		p.fail("ParsePI: PI %s never end ...", target)
	}
	data := strings.TrimLeft(p.s[p.i:p.i+end], " \t\n")
	p.i += end + 2
	return &node{kind: piNode, name: target, text: data}
}

// doctype accepts <!DOCTYPE name [SYSTEM "s" | PUBLIC "p" "s"]> (no internal subset).
func (p *xmlParser) doctype() *node {
	end := strings.IndexByte(p.s[p.i:], '>')
	if end < 0 || strings.Contains(p.s[p.i:p.i+end], "[") {
		p.fail("DOCTYPE with an internal subset is not supported")
	}
	fields := strings.Fields(p.s[p.i+len("<!DOCTYPE") : p.i+end])
	p.i += end + 1
	if len(fields) == 0 {
		p.fail("xmlParseDocTypeDecl : no DOCTYPE name !")
	}
	return &node{kind: doctypeNode, name: "<!DOCTYPE " + strings.Join(fields, " ") + ">"}
}

func (p *xmlParser) reference() string {
	end := strings.IndexByte(p.s[p.i:], ';')
	if end < 0 {
		p.fail("EntityRef: expecting ';'")
	}
	ref := p.s[p.i+1 : p.i+end]
	p.i += end + 1
	switch ref {
	case "lt":
		return "<"
	case "gt":
		return ">"
	case "amp":
		return "&"
	case "quot":
		return `"`
	case "apos":
		return "'"
	}
	if strings.HasPrefix(ref, "#") {
		var v uint64
		var err error
		if strings.HasPrefix(ref, "#x") {
			v, err = strconv.ParseUint(ref[2:], 16, 32)
		} else {
			v, err = strconv.ParseUint(ref[1:], 10, 32)
		}
		r := rune(v)
		if err != nil || !(r == 0x9 || r == 0xA || r == 0xD || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			p.fail("xmlParseCharRef: invalid xmlChar value %d", v)
		}
		return string(r)
	}
	p.fail("Entity '%s' not defined", ref)
	return ""
}

func (p *xmlParser) attrValue() string {
	if p.i >= len(p.s) || (p.s[p.i] != '"' && p.s[p.i] != '\'') {
		p.fail("AttValue: \" or ' expected")
	}
	q := p.s[p.i]
	p.i++
	var b strings.Builder
	for {
		if p.i >= len(p.s) {
			p.fail("AttValue: ' expected")
		}
		c := p.s[p.i]
		switch {
		case c == q:
			p.i++
			return b.String()
		case c == '<':
			p.fail("Unescaped '<' not allowed in attributes values")
		case c == '&':
			b.WriteString(p.reference())
		case c == '\t' || c == '\n':
			b.WriteByte(' ')
			p.i++
		default:
			b.WriteByte(c)
			p.i++
		}
	}
}

func (p *xmlParser) element() *node {
	p.i++
	startLine := 1 + strings.Count(p.s[:p.i], "\n")
	n := &node{kind: elemNode, name: p.name(), line: startLine}
	for {
		hadSpace := p.i < len(p.s) && isSpace(p.s[p.i])
		p.skipSpace()
		if strings.HasPrefix(p.s[p.i:], "/>") {
			p.i += 2
			return n
		}
		if strings.HasPrefix(p.s[p.i:], ">") {
			p.i++
			break
		}
		if !hadSpace {
			p.fail("attributes construct error")
		}
		name := p.name()
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != '=' {
			p.fail("Specification mandates value for attribute %s", name)
		}
		p.i++
		p.skipSpace()
		if _, dup := n.get(name); dup {
			p.fail("Attribute %s redefined", name)
		}
		n.attrs = append(n.attrs, attr{name, p.attrValue()})
	}
	p.content(n)
	return n
}

func (p *xmlParser) addText(n *node, t string) {
	if t == "" {
		return
	}
	if k := len(n.children); k > 0 && n.children[k-1].kind == textNode {
		n.children[k-1].text += t
		return
	}
	n.children = append(n.children, &node{kind: textNode, text: t})
}

func (p *xmlParser) content(n *node) {
	for {
		if p.i >= len(p.s) {
			p.fail("Premature end of data in tag %s", n.name)
		}
		rest := p.s[p.i:]
		switch {
		case strings.HasPrefix(rest, "</"):
			p.i += 2
			if name := p.name(); name != n.name {
				if end := strings.IndexByte(p.s[p.i:], '>'); end >= 0 {
					p.i += end + 1
				}
				lineStart := strings.LastIndexByte(p.s[:p.i], '\n') + 1
				panic(syntaxError{fmt.Sprintf("Opening and ending tag mismatch: %s line %d and %s, line %d, column %d",
					n.name, n.line, name, 1+strings.Count(p.s[:p.i], "\n"), utf8.RuneCountInString(p.s[lineStart:p.i])+1)})
			}
			p.skipSpace()
			if p.i >= len(p.s) || p.s[p.i] != '>' {
				p.fail("expected '>'")
			}
			p.i++
			return
		case strings.HasPrefix(rest, "<!--"):
			n.children = append(n.children, p.comment())
		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest, "]]>")
			if end < 0 {
				p.fail("CData section not finished")
			}
			p.addText(n, rest[9:end])
			p.i += end + 3
		case strings.HasPrefix(rest, "<?"):
			n.children = append(n.children, p.pi())
		case strings.HasPrefix(rest, "<"):
			n.children = append(n.children, p.element())
		case strings.HasPrefix(rest, "&"):
			p.addText(n, p.reference())
		default:
			end := strings.IndexAny(rest, "<&")
			if end < 0 {
				end = len(rest)
			}
			if strings.Contains(rest[:end], "]]>") {
				p.fail("Sequence ']]>' not allowed in content")
			}
			p.addText(n, rest[:end])
			p.i += end
		}
	}
}
