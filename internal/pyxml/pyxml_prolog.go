// Prolog, epilog, XML declaration, DOCTYPE and encoding of the expat-compatible parser (see
// pyxml.go). Outside the root element expat tokenizes with its DTD tokenizer: a token that
// is valid but not allowed there is a syntax error before the root and "junk after document
// element" after it.

package pyxml

import "strings"

// prolog skips everything before the root element and returns the index of its '<'.
func (p *xparser) prolog() (int, error) {
	start := p.bom // the byte order mark is skipped but counts as columns
	doctype := false
	for i := start; ; {
		if p.eof(i) {
			return i, p.failEOF()
		}
		c := p.r[i]
		var err error
		switch {
		case isS(c):
			i++
			continue
		case c != '<':
			e, kind, err := p.prologToken(i)
			if err != nil {
				return i, err
			}
			if kind == tokPERef {
				return e, p.fail(i, errPERef)
			}
			return e, p.fail(i, errSyntax)
		}
		switch c1 := p.at(i + 1); {
		case p.eof(i + 1):
			return i, p.fail(i, errUnclosed)
		case c1 == '?':
			i, err = p.pi(i, i == start)
		case c1 == '!':
			var isDoctype bool
			if i, isDoctype, err = p.bang(i, !doctype); err == nil && isDoctype {
				doctype = true
				i, err = p.doctype(i)
			}
		case p.qStart(c1):
			return i, nil
		default:
			return i, p.fail(i+1, errInvalid)
		}
		if err != nil {
			return i, err
		}
	}
}

// epilog allows only white space, comments and processing instructions after the root.
func (p *xparser) epilog(i int) error {
	for !p.eof(i) {
		c := p.r[i]
		var err error
		switch {
		case isS(c):
			i++
			continue
		case c != '<':
			if _, _, err := p.prologToken(i); err != nil {
				return err
			}
			return p.fail(i, errJunk)
		}
		switch c1 := p.at(i + 1); {
		case p.eof(i + 1):
			return p.fail(i, errUnclosed)
		case c1 == '?':
			// expat 2.9 reads a whole XML declaration here as junk, not as a misplaced one
			i, err = p.pi(i, false)
			if e, ok := err.(*Error); ok && e.Msg == errMisplaced {
				return p.fail(i, errJunk)
			}
		case c1 == '!':
			i, _, err = p.bang(i, false)
		case p.qStart(c1):
			return p.fail(i, errJunk)
		default:
			return p.fail(i+1, errInvalid)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// bang handles "<!" outside the root: a comment is skipped; "<!DOCTYPE" returns isDoctype
// with i after the keyword when allowed; any other declaration is not allowed here.
func (p *xparser) bang(s int, doctypeOK bool) (int, bool, error) {
	j := s + 2
	if p.eof(j) {
		return s, false, p.fail(s, errUnclosed)
	}
	switch c := p.r[j]; {
	case c == '-':
		i, err := p.comment(s)
		return i, false, err
	case c == '[':
		return s, false, p.notAllowed(s)
	case !p.qStart(c):
		return s, false, p.fail(j, errInvalid)
	}
	e := p.scanKeyword(j)
	if p.eof(e) {
		return s, false, p.fail(s, errUnclosed)
	}
	if c := p.r[e]; !isS(c) && c != '%' {
		return s, false, p.fail(e, errInvalid)
	}
	if doctypeOK && string(p.r[j:e]) == "DOCTYPE" {
		return e, true, nil
	}
	return s, false, p.notAllowed(s)
}

// scanKeyword scans a declaration keyword: expat takes name start characters only.
func (p *xparser) scanKeyword(i int) int {
	for i < p.n && p.qStart(p.r[i]) {
		i++
	}
	return i
}

// notAllowed is the error for a valid token at s that cannot appear outside the root.
func (p *xparser) notAllowed(s int) error {
	if p.root == nil {
		return p.fail(s, errSyntax)
	}
	return p.fail(s, errJunk)
}

const (
	tokOther = iota
	tokPERef
)

// prologToken scans one DTD token at i (not white space, not '<'): an invalid or cut token is
// returned as the error, a valid one as its end and kind.
func (p *xparser) prologToken(i int) (int, int, error) {
	c := p.r[i]
	switch {
	case c == bad || !isXMLChar(c):
		return i, 0, p.fail(i, errInvalid)
	case isNameChar(c):
		return p.afterName(p.scanName(i), ">)[%,|", true)
	case c == '"' || c == '\'':
		e := i + 1
		for ; !p.eof(e) && p.r[e] != c; e++ {
			if d := p.r[e]; d == bad || !isXMLChar(d) {
				return i, 0, p.fail(e, errInvalid)
			}
		}
		if p.eof(e) {
			return i, 0, p.fail(i, errUnclosed)
		}
		return p.afterName(e+1, ">%[", false)
	case c == '#' || c == '%':
		if p.eof(i + 1) {
			return i, 0, p.fail(i, errUnclosed)
		}
		if c == '%' && isS(p.r[i+1]) {
			return i + 1, tokOther, nil
		}
		if !isNameStart(p.r[i+1]) {
			return i, 0, p.fail(i+1, errInvalid)
		}
		e := p.scanName(i + 1)
		if c == '#' {
			return p.afterName(e, ")|>%,", false)
		}
		if p.eof(e) {
			return i, 0, p.fail(i, errUnclosed)
		}
		if p.r[e] != ';' {
			return i, 0, p.fail(e, errInvalid)
		}
		return e + 1, tokPERef, nil
	case c == ')':
		return p.afterName(i+1, ">,|)", true)
	case strings.ContainsRune("[]>(|,", c):
		return i + 1, tokOther, nil
	}
	return i, 0, p.fail(i, errInvalid)
}

// afterName checks the character ending a name-like token at e: white space, the end of
// input, one of ends, or (with occurrence) one of "*?+".
func (p *xparser) afterName(e int, ends string, occurrence bool) (int, int, error) {
	if p.eof(e) {
		return e, tokOther, nil
	}
	c := p.r[e]
	switch {
	case isS(c) || strings.ContainsRune(ends, c):
		return e, tokOther, nil
	case occurrence && strings.ContainsRune("*?+", c):
		return e + 1, tokOther, nil
	}
	return e, 0, p.fail(e, errInvalid)
}

// xmlDecl is expat's doParseXmlDecl over the declaration body r[b:end] ("?>" excluded).
func (p *xparser) xmlDecl(b, end int) error {
	i := b
	name, val, ok := p.pseudoAttr(&i, end)
	if !ok || name < 0 {
		return p.fail(i, errBadDecl)
	}
	if !p.asciiIs(name, "version") {
		return p.fail(name, errBadDecl)
	}
	if !p.versionOK(val, i-1) {
		return p.fail(val, errBadDecl)
	}
	if name, val, ok = p.pseudoAttr(&i, end); !ok {
		return p.fail(i, errBadDecl)
	}
	if name < 0 {
		return nil
	}
	if p.asciiIs(name, "encoding") {
		if c := p.ascii(val, end); !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
			return p.fail(val, errBadDecl)
		}
		p.encoding, p.encAt = string(p.r[val:i-1]), val
		if name, val, ok = p.pseudoAttr(&i, end); !ok {
			return p.fail(i, errBadDecl)
		}
		if name < 0 {
			return nil
		}
	}
	if !p.asciiIs(name, "standalone") {
		return p.fail(name, errBadDecl)
	}
	if v := string(p.r[val : i-1]); v != "yes" && v != "no" {
		return p.fail(val, errBadDecl)
	}
	for isS(p.ascii(i, end)) {
		i++
	}
	if i != end {
		return p.fail(i, errBadDecl)
	}
	return nil
}

// ascii is expat's toAscii: the character at i, -1 for non-ASCII or at end.
func (p *xparser) ascii(i, end int) rune {
	if i >= end || p.r[i] < 0 || p.r[i] >= 0x80 {
		return -1
	}
	return p.r[i]
}

// asciiIs reports whether the name starting at i is exactly kw (it ends at '=' or space).
func (p *xparser) asciiIs(i int, kw string) bool {
	for _, c := range kw {
		if p.at(i) != c {
			return false
		}
		i++
	}
	c := p.at(i)
	return c == '=' || isS(c)
}

// versionOK is expat's check of the version value: "1." followed by digits.
func (p *xparser) versionOK(v, e int) bool {
	s := string(p.r[v:e])
	if len(s) < 3 || s[:2] != "1." {
		return false
	}
	return strings.Trim(s[2:], "0123456789") == ""
}

// pseudoAttr is expat's parsePseudoAttribute: name < 0 means none left; on failure *i is the
// bad position.
func (p *xparser) pseudoAttr(i *int, end int) (name, val int, ok bool) {
	ptr := *i
	if ptr == end {
		return -1, 0, true
	}
	if !isS(p.ascii(ptr, end)) {
		return 0, 0, false
	}
	for ptr++; isS(p.ascii(ptr, end)); ptr++ {
	}
	if ptr == end {
		*i = ptr
		return -1, 0, true
	}
	name = ptr
	for {
		c := p.ascii(ptr, end)
		if c == -1 {
			*i = ptr
			return 0, 0, false
		}
		if c == '=' {
			break
		}
		if isS(c) {
			for ptr++; isS(p.ascii(ptr, end)); ptr++ {
			}
			if p.ascii(ptr, end) != '=' {
				*i = ptr
				return 0, 0, false
			}
			break
		}
		ptr++
	}
	if ptr == name {
		*i = ptr
		return 0, 0, false
	}
	for ptr++; isS(p.ascii(ptr, end)); ptr++ {
	}
	q := p.ascii(ptr, end)
	if q != '"' && q != '\'' {
		*i = ptr
		return 0, 0, false
	}
	val = ptr + 1
	for ptr = val; ; ptr++ {
		c := p.ascii(ptr, end)
		if c == q {
			break
		}
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '.' || c == '-' || c == '_') {
			*i = ptr
			return 0, 0, false
		}
	}
	*i = ptr + 1
	return name, val, true
}
