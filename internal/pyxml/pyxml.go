// Package pyxml is an expat-compatible XML parser shared by the ports that check or read
// XML the way their Python tool does (bog-cycles, bog-write, station-modules).
//
// It reproduces the well-formedness verdict and the error text of Python's pyexpat (expat
// 2.x): "<message>: line L, column C" with expat's token-start positions, so a malformed
// document fails with the same words as the Python tool. encoding/xml cannot do this: its
// messages, its positions (after the token, line only) and its leniency (a second root, a
// misplaced XML declaration, duplicate attributes) all differ. testdata/xmlerrors.json, made
// by gen_xmlerrors.py from expat itself, is the parity suite.
package pyxml

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Mode selects the Python call being reproduced.
type Mode struct {
	// ETree is xml.etree.ElementTree.fromstring(bytes): namespace processing, encoding from
	// the declaration. Without it, pyexpat ParserCreate().Parse(str): no namespaces, UTF-8.
	ETree bool
}

// Element is an ElementTree element: Tag is "{uri}local" for a namespaced name.
type Element struct {
	Tag      string
	Attrs    []Attr // document order, xmlns declarations removed in ETree mode
	Children []*Element
}

// Attr is one attribute: Name is "{uri}local" for a namespaced name in ETree mode.
type Attr struct{ Name, Value string }

// Get is Element.get(name): the value and whether the attribute exists.
func (e *Element) Get(name string) (string, bool) {
	for _, a := range e.Attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// Error is expat's error: ExpatError / ParseError str(); Line is 1-based, Col 0-based.
type Error struct {
	Msg       string
	Line, Col int
}

func (e *Error) Error() string { return fmt.Sprintf("%s: line %d, column %d", e.Msg, e.Line, e.Col) }

const (
	errNoElement  = "no element found"
	errInvalid    = "not well-formed (invalid token)"
	errUnclosed   = "unclosed token"
	errMismatch   = "mismatched tag"
	errDupAttr    = "duplicate attribute"
	errJunk       = "junk after document element"
	errUndefined  = "undefined entity"
	errBadCharRef = "reference to invalid character number"
	errCDATA      = "unclosed CDATA section"
	errMisplaced  = "XML or text declaration not at start of entity"
	errBadDecl    = "XML declaration not well-formed"
	errSyntax     = "syntax error"
	errUnbound    = "unbound prefix"
	errUndeclare  = "must not undeclare prefix"
	errPERef      = "illegal parameter entity reference"
	errEncoding   = "unknown encoding"
	bad           = -1 // an undecodable byte in the rune slice
)

const xmlNS = "http://www.w3.org/XML/1998/namespace"

var predefined = map[string]rune{"lt": '<', "gt": '>', "amp": '&', "apos": '\'', "quot": '"'}

type xparser struct {
	r        []rune
	n        int
	mode     Mode
	stack    []*openElem
	root     *Element
	entities map[string]string // general entities of the internal DTD subset
	extDTD   bool              // external DTD subset or PE reference: undefined entities are skipped
	encoding string            // the declaration's encoding name
}

type openElem struct {
	qname string
	elem  *Element
	ns    map[string]string // prefix -> uri in scope ("" is the default namespace)
}

// Parse checks data the way the selected Python call does and returns its element tree.
func Parse(data []byte, mode Mode) (*Element, error) {
	p := &xparser{r: decodeUTF8(data), mode: mode}
	p.n = len(p.r)
	if mode.ETree {
		if err := p.applyEncoding(data); err != nil {
			return nil, err
		}
	}
	if err := p.document(); err != nil {
		return nil, err
	}
	return p.root, nil
}

// decodeUTF8 decodes data, one bad rune per undecodable byte (expat stops at the first).
func decodeUTF8(data []byte) []rune {
	out := make([]rune, 0, len(data))
	for len(data) > 0 {
		c, size := utf8.DecodeRune(data)
		if c == utf8.RuneError && size == 1 {
			c = bad
		}
		out = append(out, c)
		data = data[size:]
	}
	return out
}

func (p *xparser) at(i int) rune {
	if i < p.n {
		return p.r[i]
	}
	return bad - 1 // end of input
}

func (p *xparser) eof(i int) bool { return i >= p.n }

// fail builds the error at rune index pos: expat counts lines at LF, CR and CRLF and columns
// in characters from the last line break.
func (p *xparser) fail(pos int, msg string) error {
	line, col := 1, 0
	for i := 0; i < pos && i < p.n; i++ {
		switch p.r[i] {
		case '\n':
			line, col = line+1, 0
		case '\r':
			line, col = line+1, 0
			if i+1 < pos && p.at(i+1) == '\n' {
				i++
			}
		default:
			col++
		}
	}
	return &Error{Msg: msg, Line: line, Col: col}
}

// failEOF is "no element found" at the end of input. Inside the root element a trailing CR
// is still a pending token (it may start a CRLF), so expat reports the position before it.
func (p *xparser) failEOF() error {
	pos := p.n
	if len(p.stack) > 0 && pos > 0 && p.r[pos-1] == '\r' {
		pos--
	}
	return p.fail(pos, errNoElement)
}

func isXMLChar(c rune) bool {
	return c == 0x9 || c == 0xA || c == 0xD || 0x20 <= c && c <= 0xD7FF ||
		0xE000 <= c && c <= 0xFFFD || 0x10000 <= c && c <= 0x10FFFF
}

func isS(c rune) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// isNameStart and isNameChar follow XML 1.0 (4th edition) as expat does: BMP only.
func isNameStart(c rune) bool {
	if c < utf8.RuneSelf {
		return c == '_' || c == ':' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
	}
	return c <= 0xFFFF && (unicode.IsLetter(c) && !unicode.Is(unicode.Lm, c) || unicode.Is(unicode.Nl, c))
}

func isNameChar(c rune) bool {
	if c < utf8.RuneSelf {
		return isNameStart(c) || c == '-' || c == '.' || '0' <= c && c <= '9'
	}
	return isNameStart(c) || c <= 0xFFFF && (c == 0xB7 || unicode.In(c, unicode.Mn, unicode.Mc, unicode.Me, unicode.Nd, unicode.Lm))
}

// scanName returns the end of the name chars starting at i (a DTD name: colons allowed).
func (p *xparser) scanName(i int) int {
	for i < p.n && isNameChar(p.r[i]) {
		i++
	}
	return i
}

// qStart reports a character that may start an element, attribute, PI or entity name: with
// namespace processing expat does not let a name start with ':'.
func (p *xparser) qStart(c rune) bool { return isNameStart(c) && !(p.mode.ETree && c == ':') }

// scanNC scans a PI target or entity name: with namespace processing it stops at ':'.
func (p *xparser) scanNC(i int) int {
	for i < p.n && isNameChar(p.r[i]) && !(p.mode.ETree && p.r[i] == ':') {
		i++
	}
	return i
}

// scanQName scans an element or attribute name: with namespace processing at most one ':',
// followed by a name start character.
func (p *xparser) scanQName(i int) (int, error) {
	e := p.scanNC(i)
	if !p.mode.ETree || p.at(e) != ':' {
		return e, nil
	}
	if p.eof(e + 1) {
		return e + 1, nil
	}
	if !p.qStart(p.r[e+1]) {
		return e, p.fail(e+1, errInvalid)
	}
	if e = p.scanNC(e + 1); p.at(e) == ':' {
		return e, p.fail(e, errInvalid)
	}
	return e, nil
}

func (p *xparser) document() error {
	i, err := p.prolog()
	if err != nil {
		return err
	}
	if i, err = p.element(i); err != nil {
		return err
	}
	return p.epilog(i)
}

// element parses the root element starting at its '<' and returns the index after it.
func (p *xparser) element(i int) (int, error) {
	var err error
	if i, err = p.startTag(i); err != nil || len(p.stack) == 0 {
		return i, err
	}
	for {
		if p.eof(i) {
			return i, p.failEOF()
		}
		switch c := p.r[i]; {
		case c == '<':
			if i, err = p.markup(i); err != nil || len(p.stack) == 0 {
				return i, err
			}
		case c == '&':
			if i, err = p.contentRef(i); err != nil {
				return i, err
			}
		case c == ']' && p.at(i+1) == ']' && p.at(i+2) == '>':
			return i, p.fail(i+2, errInvalid)
		case c == bad || !isXMLChar(c):
			return i, p.fail(i, errInvalid)
		default:
			i++
		}
	}
}

// markup is a '<' inside the root element.
func (p *xparser) markup(s int) (int, error) {
	switch c := p.at(s + 1); {
	case p.eof(s + 1):
		return s, p.fail(s, errUnclosed)
	case c == '/':
		return p.endTag(s)
	case c == '!':
		return p.contentBang(s)
	case c == '?':
		return p.pi(s, false)
	case p.qStart(c):
		return p.startTag(s)
	default:
		return s, p.fail(s+1, errInvalid)
	}
}

func (p *xparser) contentBang(s int) (int, error) {
	i := s + 2
	switch {
	case p.eof(i):
		return s, p.fail(s, errUnclosed)
	case p.r[i] == '-':
		return p.comment(s)
	case p.r[i] != '[':
		return s, p.fail(i, errInvalid)
	}
	for _, c := range "[CDATA[" {
		if p.eof(i) {
			return s, p.fail(s, errUnclosed)
		}
		if p.r[i] != c {
			return s, p.fail(i, errInvalid)
		}
		i++
	}
	i0 := i
	for ; !p.eof(i); i++ {
		c := p.r[i]
		if c == ']' && p.at(i+1) == ']' && p.at(i+2) == '>' {
			return i + 3, nil
		}
		if c == bad || !isXMLChar(c) {
			return s, p.fail(i, errInvalid)
		}
	}
	// a trailing CR or "]"/"]]" is a pending token: expat reports the position before it
	pos := p.n
	if p.r[pos-1] == '\r' {
		pos--
	} else {
		for k := 0; k < 2 && pos > i0 && p.r[pos-1] == ']'; k++ {
			pos--
		}
	}
	return s, p.fail(pos, errCDATA)
}

// comment scans "<!--...-->" from its '<'.
func (p *xparser) comment(s int) (int, error) {
	i := s + 3
	if p.eof(i) {
		return s, p.fail(s, errUnclosed)
	}
	if p.r[i] != '-' {
		return s, p.fail(i, errInvalid)
	}
	for i++; !p.eof(i); i++ {
		c := p.r[i]
		if c == bad || !isXMLChar(c) {
			return s, p.fail(i, errInvalid)
		}
		if c == '-' && p.at(i+1) == '-' {
			switch {
			case p.eof(i + 2):
				return s, p.fail(s, errUnclosed)
			case p.r[i+2] != '>':
				return s, p.fail(i+2, errInvalid)
			}
			return i + 3, nil
		}
	}
	return s, p.fail(s, errUnclosed)
}

// pi scans "<?target ...?>" from its '<'; atStart allows the XML declaration.
func (p *xparser) pi(s int, atStart bool) (int, error) {
	i := s + 2
	if p.eof(i) {
		return s, p.fail(s, errUnclosed)
	}
	if !p.qStart(p.r[i]) {
		return s, p.fail(i, errInvalid)
	}
	e := p.scanNC(i)
	target := string(p.r[i:e])
	if len(target) == 3 && target != "xml" && (target[0]|0x20) == 'x' && (target[1]|0x20) == 'm' && (target[2]|0x20) == 'l' {
		return s, p.fail(e, errInvalid)
	}
	body := e
	switch c := p.at(e); {
	case p.eof(e):
		return s, p.fail(s, errUnclosed)
	case c == '?':
		if p.eof(e + 1) {
			return s, p.fail(s, errUnclosed)
		}
		if p.r[e+1] != '>' {
			return s, p.fail(e+1, errInvalid)
		}
		e += 2
	case !isS(c):
		return s, p.fail(e, errInvalid)
	default:
		for e++; ; e++ {
			if p.eof(e) {
				return s, p.fail(s, errUnclosed)
			}
			if c := p.r[e]; c == bad || !isXMLChar(c) {
				return s, p.fail(e, errInvalid)
			}
			if p.r[e] == '?' && p.at(e+1) == '>' {
				e += 2
				break
			}
		}
	}
	if target == "xml" {
		if !atStart {
			return s, p.fail(s, errMisplaced)
		}
		return e, p.xmlDecl(body, e-2)
	}
	return e, nil
}
