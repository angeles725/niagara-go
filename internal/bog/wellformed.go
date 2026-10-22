package bog

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pyxml"
)

// WellFormedError is a structural fault of a BOG text: expat's message and its position (line
// from 1, column in characters from 0, as expat counts them).
type WellFormedError struct {
	Msg       string
	Line, Col int
}

func (e *WellFormedError) Error() string {
	return fmt.Sprintf("%s: line %d, column %d", e.Msg, e.Line, e.Col)
}

// CheckWellFormed reports the first well-formedness fault of an XML text, nil when there is none,
// with the verdict and the message of bog-cycles (pyexpat ParserCreate().Parse(str), reproduced
// by internal/pyxml Mode{}): issue #170.
//
// A fast single pass (about 20 ms on the 7.1 MB HARBOR file.xml, where pyxml takes over a second)
// accepts a text only when it is certainly well-formed: one root element, names, quoted attribute
// values without '<' or a repeated name, references to the five predefined entities or to legal
// characters, only XML characters, comments without "--", CDATA sections and processing
// instructions terminated, and an XML declaration of the usual shape. Anything else, including
// any DOCTYPE (its entity declarations decide which references are defined) and non-ASCII names,
// goes to pyxml, whose verdict and message are returned; a fault therefore costs one pyxml parse.
func CheckWellFormed(text string) error {
	c := wfScanner{s: text}
	if c.run() {
		return nil
	}
	_, err := pyxml.Parse([]byte(text), pyxml.Mode{})
	var pe *pyxml.Error
	if errors.As(err, &pe) {
		return &WellFormedError{Msg: pe.Msg, Line: pe.Line, Col: pe.Col}
	}
	return err
}

// ReadCheckedText is ReadText that fails closed: a text CheckWellFormed rejects is a *ReadError
// "<path>: not well-formed XML (<message>: line L, column C)", the verdict bog-cycles prints.
func ReadCheckedText(path string) (string, error) {
	text, err := ReadText(path)
	if err != nil {
		return "", err
	}
	if err := CheckWellFormed(text); err != nil {
		return "", readErr(err, "%s: not well-formed XML (%s)", path, err)
	}
	return text, nil
}

// Byte classes of the fast pass.
const (
	bPlain    = iota
	bSpecial  // a byte the scanning loop must look at: '<', '&', ']', a quote
	bControl  // a C0 control other than TAB, LF and CR: never an XML character
	bNonASCII // the start or continuation of a multi-byte UTF-8 sequence
)

var wfClass = func() (t [256]uint8) {
	for b := 0; b < 0x20; b++ {
		t[b] = bControl
	}
	t['\t'], t['\n'], t['\r'] = bPlain, bPlain, bPlain
	for _, b := range "<&]'\"" {
		t[b] = bSpecial
	}
	for b := 0x80; b < 0x100; b++ {
		t[b] = bNonASCII
	}
	return t
}()

// wfScanner is the fast pass: run reports whether the text is certainly well-formed. A false
// result is not a verdict; CheckWellFormed then asks pyxml.
type wfScanner struct {
	s     string
	stack []string // names of the open elements
	attrs []string // attribute names of the current start tag (reset per tag)
}

func (c *wfScanner) run() bool {
	s := c.s
	i := 0
	if strings.HasPrefix(s, "\xef\xbb\xbf") {
		i = len("\xef\xbb\xbf")
	}
	if strings.HasPrefix(s[i:], "<?xml") && len(s) > i+5 && (isXMLSpace(s[i+5]) || s[i+5] == '?') {
		var ok bool
		if i, ok = xmlDecl(s, i); !ok {
			return false
		}
	}
	// prolog: white space, comments and processing instructions before the root
	for {
		i = skipSpace(s, i)
		if i >= len(s) || s[i] != '<' || i+1 >= len(s) {
			return false
		}
		switch b := s[i+1]; {
		case b == '?':
			if i = c.pi(i); i < 0 {
				return false
			}
		case strings.HasPrefix(s[i:], "<!--"):
			if i = c.comment(i); i < 0 {
				return false
			}
		case isNameStartASCII(b):
			if i = c.element(i); i < 0 {
				return false
			}
			return c.epilog(i)
		default:
			return false // a DOCTYPE, or a fault
		}
	}
}

// element checks the root element starting at its '<' and returns the index after it, -1 when
// the text is not certainly well-formed.
func (c *wfScanner) element(i int) int {
	s := c.s
	if i = c.startTag(i); i < 0 || len(c.stack) == 0 {
		return i
	}
	for i < len(s) {
		b := s[i]
		switch wfClass[b] {
		case bPlain:
			i++
			continue
		case bControl:
			return -1
		case bNonASCII:
			if i = xmlChar(s, i); i < 0 {
				return -1
			}
			continue
		}
		switch b {
		case '<':
			if i = c.markup(i); i < 0 || len(c.stack) == 0 {
				return i
			}
		case '&':
			if i = reference(s, i); i < 0 {
				return -1
			}
		case ']':
			if strings.HasPrefix(s[i:], "]]>") {
				return -1
			}
			i++
		default: // a quote in character data
			i++
		}
	}
	return -1 // the root element is not closed
}

// markup is a '<' inside the root element.
func (c *wfScanner) markup(i int) int {
	s := c.s
	if i+1 >= len(s) {
		return -1
	}
	switch b := s[i+1]; {
	case b == '/':
		return c.endTag(i)
	case b == '?':
		return c.pi(i)
	case strings.HasPrefix(s[i:], "<!--"):
		return c.comment(i)
	case strings.HasPrefix(s[i:], "<![CDATA["):
		return chars(s, i+len("<![CDATA["), "]]>")
	case isNameStartASCII(b):
		return c.startTag(i)
	}
	return -1
}

func (c *wfScanner) epilog(i int) bool {
	s := c.s
	for {
		if i = skipSpace(s, i); i >= len(s) {
			return true
		}
		switch {
		case strings.HasPrefix(s[i:], "<?"):
			i = c.pi(i)
		case strings.HasPrefix(s[i:], "<!--"):
			i = c.comment(i)
		default:
			return false
		}
		if i < 0 {
			return false
		}
	}
}

func (c *wfScanner) startTag(i int) int {
	s := c.s
	nameEnd := scanNameASCII(s, i+1)
	if nameEnd == i+1 {
		return -1
	}
	name := s[i+1 : nameEnd]
	c.attrs = c.attrs[:0]
	k := nameEnd
	for {
		sp := skipSpace(s, k)
		if sp >= len(s) {
			return -1
		}
		switch s[sp] {
		case '>':
			c.stack = append(c.stack, name)
			return sp + 1
		case '/':
			if sp+1 >= len(s) || s[sp+1] != '>' {
				return -1
			}
			return sp + 2
		}
		if sp == k { // an attribute must follow white space
			return -1
		}
		if k = c.attribute(sp); k < 0 {
			return -1
		}
	}
}

// attribute checks name="value" (or 'value') at k and records the name.
func (c *wfScanner) attribute(k int) int {
	s := c.s
	nameEnd := scanNameASCII(s, k)
	if nameEnd == k {
		return -1
	}
	name := s[k:nameEnd]
	for _, seen := range c.attrs {
		if seen == name {
			return -1
		}
	}
	c.attrs = append(c.attrs, name)
	eq := skipSpace(s, nameEnd)
	if eq >= len(s) || s[eq] != '=' {
		return -1
	}
	q := skipSpace(s, eq+1)
	if q >= len(s) || s[q] != '"' && s[q] != '\'' {
		return -1
	}
	quote := s[q]
	for i := q + 1; i < len(s); {
		b := s[i]
		switch wfClass[b] {
		case bPlain:
			i++
			continue
		case bControl:
			return -1
		case bNonASCII:
			if i = xmlChar(s, i); i < 0 {
				return -1
			}
			continue
		}
		switch b {
		case quote:
			return i + 1
		case '<':
			return -1
		case '&':
			if i = reference(s, i); i < 0 {
				return -1
			}
		default: // ']' or the other quote
			i++
		}
	}
	return -1
}

func (c *wfScanner) endTag(i int) int {
	s := c.s
	nameEnd := scanNameASCII(s, i+2)
	if nameEnd == i+2 {
		return -1
	}
	gt := skipSpace(s, nameEnd)
	if gt >= len(s) || s[gt] != '>' {
		return -1
	}
	if len(c.stack) == 0 || c.stack[len(c.stack)-1] != s[i+2:nameEnd] {
		return -1
	}
	c.stack = c.stack[:len(c.stack)-1]
	return gt + 1
}

// comment checks "<!--...-->" at i: "--" may only close it.
func (c *wfScanner) comment(i int) int {
	s := c.s
	start := i + len("<!--")
	k := strings.Index(s[start:], "--")
	if k < 0 || start+k+2 >= len(s) || s[start+k+2] != '>' {
		return -1
	}
	if !charsOK(s[start : start+k]) {
		return -1
	}
	return start + k + 3
}

// pi checks a processing instruction "<?target ...?>" at i other than the XML declaration.
func (c *wfScanner) pi(i int) int {
	s := c.s
	nameEnd := scanNameASCII(s, i+2)
	if nameEnd == i+2 || nameEnd >= len(s) {
		return -1
	}
	if t := s[i+2 : nameEnd]; len(t) == 3 && strings.EqualFold(t, "xml") {
		return -1 // a misplaced declaration, or a reserved target
	}
	switch b := s[nameEnd]; {
	case b == '?':
		if nameEnd+1 < len(s) && s[nameEnd+1] == '>' {
			return nameEnd + 2
		}
		return -1
	case !isXMLSpace(b):
		return -1
	}
	return chars(s, nameEnd, "?>")
}

// chars returns the index after the first terminator at or after i when every character before
// it is an XML character, else -1.
func chars(s string, i int, terminator string) int {
	k := strings.Index(s[i:], terminator)
	if k < 0 || !charsOK(s[i:i+k]) {
		return -1
	}
	return i + k + len(terminator)
}

func charsOK(s string) bool {
	for i := 0; i < len(s); {
		switch wfClass[s[i]] {
		case bControl:
			return false
		case bNonASCII:
			if i = xmlChar(s, i); i < 0 {
				return false
			}
		default:
			i++
		}
	}
	return true
}

// xmlChar decodes the non-ASCII character at i and returns the index after it, -1 when it is an
// undecodable byte or not an XML character (U+FFFE, U+FFFF).
func xmlChar(s string, i int) int {
	r, size := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && size == 1 || r == 0xFFFE || r == 0xFFFF {
		return -1
	}
	return i + size
}

func isNameStartASCII(b byte) bool { return b == '_' || b == ':' || isASCIILetter(b) }

// scanNameASCII returns the end of the ASCII XML name at k (k itself when there is none). A name
// running into a non-ASCII character stops there, and the caller's next check fails, so such
// names are left to pyxml (which applies expat's Unicode name tables).
func scanNameASCII(s string, k int) int {
	if k >= len(s) || !isNameStartASCII(s[k]) {
		return k
	}
	for k++; k < len(s); k++ {
		b := s[k]
		if !isNameStartASCII(b) && b != '-' && b != '.' && !('0' <= b && b <= '9') {
			break
		}
	}
	return k
}

func isXMLSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func skipSpace(s string, k int) int {
	for k < len(s) && isXMLSpace(s[k]) {
		k++
	}
	return k
}
