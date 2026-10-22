package bog

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Expat's messages for the faults CheckWellFormed detects, so a caller prints the same verdict
// wording as bog-cycles and bog-write (internal/pyxml).
const (
	wfNoElement = "no element found"
	wfUnclosed  = "unclosed token"
	wfInvalid   = "not well-formed (invalid token)"
	wfMismatch  = "mismatched tag"
	wfJunk      = "junk after document element"
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

// CheckWellFormed reports the first structural fault of an XML text, nil when there is none: one
// root element, every start tag closed by '>' with its attribute values quoted, end tags matching
// the open element, and comments, CDATA sections, processing instructions and a DOCTYPE
// terminated. It is the fast single-pass check Load needs to fail closed on a truncated or
// damaged bog (a HARBOR 7.1 MB file.xml in a few ms, where internal/pyxml takes over a second);
// it does not validate names, references, namespaces or duplicate attributes, which pyxml does
// for the callers that need expat's full verdict (bog-cycles, bog-write).
func CheckWellFormed(text string) error {
	c := wfChecker{s: text}
	return c.run()
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

type wfChecker struct {
	s     string
	stack []string // names of the open elements
	root  bool     // the root element has started
}

func (c *wfChecker) fail(off int, msg string) error {
	line := 1 + strings.Count(c.s[:off], "\n")
	lineStart := strings.LastIndexByte(c.s[:off], '\n') + 1
	return &WellFormedError{Msg: msg, Line: line, Col: utf8.RuneCountInString(c.s[lineStart:off])}
}

func (c *wfChecker) run() error {
	s := c.s
	i := 0
	if strings.HasPrefix(s, "\xef\xbb\xbf") {
		i = len("\xef\xbb\xbf")
	}
	for {
		j := strings.IndexByte(s[i:], '<')
		end := len(s)
		if j >= 0 {
			end = i + j
		}
		if len(c.stack) == 0 {
			if k := firstNonSpace(s, i, end); k >= 0 {
				if c.root {
					return c.fail(k, wfJunk)
				}
				return c.fail(k, wfInvalid)
			}
		}
		if j < 0 {
			break
		}
		next, err := c.markup(end)
		if err != nil {
			return err
		}
		i = next
	}
	if len(c.stack) > 0 || !c.root {
		return c.fail(len(s), wfNoElement)
	}
	return nil
}

// markup checks the construct starting with '<' at i and returns the index after it.
func (c *wfChecker) markup(i int) (int, error) {
	s := c.s
	rest := s[i:]
	switch {
	case strings.HasPrefix(rest, "<?"):
		return c.until(i, "?>")
	case strings.HasPrefix(rest, "<!--"):
		return c.until(i, "-->")
	case strings.HasPrefix(rest, "<![CDATA["):
		if len(c.stack) == 0 {
			return 0, c.fail(i, c.outsideRoot())
		}
		return c.until(i, "]]>")
	case strings.HasPrefix(rest, "<!DOCTYPE"):
		if c.root {
			return 0, c.fail(i, c.outsideRoot())
		}
		if k := strings.IndexAny(rest, "[>"); k >= 0 && rest[k] == '[' {
			return c.until(i, "]")
		}
		return c.until(i, ">")
	case strings.HasPrefix(rest, "</"):
		return c.endTag(i)
	}
	return c.startTag(i)
}

func (c *wfChecker) outsideRoot() string {
	if c.root {
		return wfJunk
	}
	return wfInvalid
}

// until returns the index after the first terminator at or after i, or an unclosed-token fault.
func (c *wfChecker) until(i int, terminator string) (int, error) {
	k := strings.Index(c.s[i+1:], terminator)
	if k < 0 {
		return 0, c.fail(i, wfUnclosed)
	}
	return i + 1 + k + len(terminator), nil
}

func (c *wfChecker) startTag(i int) (int, error) {
	s := c.s
	if len(c.stack) == 0 && c.root {
		return 0, c.fail(i, wfJunk)
	}
	nameEnd := scanName(s, i+1)
	if nameEnd == i+1 {
		return 0, c.fail(i, c.eofOr(i+1, wfInvalid))
	}
	name := s[i+1 : nameEnd]
	k := nameEnd
	for {
		sp := skipSpace(s, k)
		if sp >= len(s) {
			return 0, c.fail(i, wfUnclosed)
		}
		switch s[sp] {
		case '>':
			c.root = true
			c.stack = append(c.stack, name)
			return sp + 1, nil
		case '/':
			if sp+1 >= len(s) {
				return 0, c.fail(i, wfUnclosed)
			}
			if s[sp+1] != '>' {
				return 0, c.fail(sp, wfInvalid)
			}
			c.root = true
			return sp + 2, nil
		}
		if sp == k { // an attribute must follow white space
			return 0, c.fail(sp, wfInvalid)
		}
		next, err := c.attribute(i, sp)
		if err != nil {
			return 0, err
		}
		k = next
	}
}

// attribute checks name="value" (or 'value') at k inside the tag opened at tagStart.
func (c *wfChecker) attribute(tagStart, k int) (int, error) {
	s := c.s
	nameEnd := scanName(s, k)
	if nameEnd == k {
		return 0, c.fail(k, wfInvalid)
	}
	eq := skipSpace(s, nameEnd)
	if eq >= len(s) {
		return 0, c.fail(tagStart, wfUnclosed)
	}
	if s[eq] != '=' {
		return 0, c.fail(eq, wfInvalid)
	}
	q := skipSpace(s, eq+1)
	if q >= len(s) {
		return 0, c.fail(tagStart, wfUnclosed)
	}
	if s[q] != '"' && s[q] != '\'' {
		return 0, c.fail(q, wfInvalid)
	}
	closeAt := strings.IndexByte(s[q+1:], s[q])
	if closeAt < 0 {
		return 0, c.fail(tagStart, wfUnclosed)
	}
	value := s[q+1 : q+1+closeAt]
	if lt := strings.IndexByte(value, '<'); lt >= 0 {
		return 0, c.fail(q+1+lt, wfInvalid)
	}
	return q + 1 + closeAt + 1, nil
}

func (c *wfChecker) endTag(i int) (int, error) {
	s := c.s
	nameEnd := scanName(s, i+2)
	if nameEnd == i+2 {
		return 0, c.fail(i, c.eofOr(i+2, wfInvalid))
	}
	gt := skipSpace(s, nameEnd)
	if gt >= len(s) {
		return 0, c.fail(i, wfUnclosed)
	}
	if s[gt] != '>' {
		return 0, c.fail(gt, wfInvalid)
	}
	if len(c.stack) == 0 || c.stack[len(c.stack)-1] != s[i+2:nameEnd] {
		return 0, c.fail(i+2, c.mismatchOrJunk())
	}
	c.stack = c.stack[:len(c.stack)-1]
	return gt + 1, nil
}

func (c *wfChecker) mismatchOrJunk() string {
	if len(c.stack) == 0 {
		return wfJunk
	}
	return wfMismatch
}

// eofOr is unclosed-token when the text ends at k, else msg.
func (c *wfChecker) eofOr(k int, msg string) string {
	if k >= len(c.s) {
		return wfUnclosed
	}
	return msg
}

// scanName returns the end of the XML name at k (k itself when there is none). Like expat, a name
// cannot start with a digit, '-' or '.'; non-ASCII characters are accepted as name characters.
func scanName(s string, k int) int {
	start := k
	for k < len(s) {
		b := s[k]
		isStart := b == '_' || b == ':' || isASCIILetter(b) || b >= utf8.RuneSelf
		if !isStart && (k == start || !(b == '-' || b == '.' || '0' <= b && b <= '9')) {
			break
		}
		k++
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

// firstNonSpace is the index of the first non-space byte of s[from:to], -1 when there is none.
func firstNonSpace(s string, from, to int) int {
	for k := from; k < to; k++ {
		if !isXMLSpace(s[k]) {
			return k
		}
	}
	return -1
}
