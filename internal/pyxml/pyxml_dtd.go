// DOCTYPE and declared encoding of the expat-compatible parser (see pyxml.go). Both are
// best effort: station BOGs and module.xml files carry neither a DOCTYPE nor a non-UTF-8
// encoding, so only the shapes expat accepts are modelled and the rest fails as a syntax
// error.

package pyxml

import "strings"

func (p *xparser) skipS(i int) int {
	for !p.eof(i) && isS(p.r[i]) {
		i++
	}
	return i
}

// doctype parses the rest of "<!DOCTYPE name ExternalID? [subset]? >" from after the keyword.
func (p *xparser) doctype(i int) (int, error) {
	if i = p.skipS(i); p.eof(i) {
		return i, p.failEOF()
	}
	if !isNameStart(p.r[i]) {
		return p.unexpected(i)
	}
	e, kind, err := p.prologToken(i)
	if err != nil {
		return i, err
	}
	if kind == tokNmtoken {
		return i, p.fail(i, errSyntax) // a DOCTYPE name is a name, not an NMTOKEN
	}
	if i = p.skipS(e); p.eof(i) {
		return i, p.failEOF()
	}
	if isNameStart(p.r[i]) {
		e = p.scanName(i)
		kw, lits := string(p.r[i:e]), 0
		switch kw {
		case "SYSTEM":
			lits = 1
		case "PUBLIC":
			lits = 2
		default:
			return i, p.fail(i, errSyntax)
		}
		i = e
		for ; lits > 0; lits-- {
			if i = p.skipS(i); p.eof(i) {
				return i, p.failEOF()
			}
			if c := p.r[i]; c != '"' && c != '\'' {
				return p.unexpected(i)
			}
			if i, _, err = p.prologToken(i); err != nil {
				return i, err
			}
		}
		p.extDTD = true
		i = p.skipS(i)
	}
	if !p.eof(i) && p.r[i] == '[' {
		if i, err = p.subset(i + 1); err != nil {
			return i, err
		}
		i = p.skipS(i)
	}
	if p.eof(i) {
		return i, p.failEOF()
	}
	if p.r[i] != '>' {
		return p.unexpected(i)
	}
	return i + 1, nil
}

// unexpected is a token the DOCTYPE grammar does not allow at i.
func (p *xparser) unexpected(i int) (int, error) {
	if p.r[i] != '<' {
		if _, _, err := p.prologToken(i); err != nil {
			return i, err
		}
	}
	return i, p.fail(i, errSyntax)
}

// subset skips the internal subset up to its ']', recording general internal entities.
func (p *xparser) subset(i int) (int, error) {
	for {
		if i = p.skipS(i); p.eof(i) {
			return i, p.failEOF()
		}
		var err error
		switch c := p.r[i]; c {
		case ']':
			return i + 1, nil
		case '%':
			e, kind, err := p.prologToken(i)
			if err != nil {
				return i, err
			}
			if kind != tokPERef {
				return i, p.fail(i, errSyntax)
			}
			p.extDTD, i = true, e
			continue
		case '<':
			i, err = p.markupDecl(i)
		default:
			_, err = p.unexpected(i)
		}
		if err != nil {
			return i, err
		}
	}
}

// markupDecl is a comment, a PI or a <!KEYWORD ...> declaration of the internal subset.
func (p *xparser) markupDecl(s int) (int, error) {
	switch c := p.at(s + 1); {
	case p.eof(s + 1):
		return s, p.fail(s, errUnclosed)
	case c == '?':
		return p.pi(s, false)
	case c != '!':
		return s, p.fail(s+1, errInvalid)
	}
	j := s + 2
	if p.at(j) == '-' {
		return p.comment(s)
	}
	if p.eof(j) {
		return s, p.fail(s, errUnclosed)
	}
	if !p.keywordChar(p.r[j]) {
		return s, p.fail(j, errInvalid)
	}
	e := p.scanKeyword(j)
	if p.eof(e) {
		return s, p.fail(s, errUnclosed)
	}
	if c := p.r[e]; !isS(c) && c != '%' {
		return s, p.fail(e, errInvalid)
	}
	if string(p.r[j:e]) == "ENTITY" {
		p.entityDecl(e)
	}
	return p.skipDecl(e)
}

// entityDecl records <!ENTITY name 'value'> (first definition wins; the replacement text
// is taken literally).
func (p *xparser) entityDecl(i int) {
	i = p.skipS(i)
	if p.at(i) == '%' {
		return
	}
	if p.eof(i) || !isNameStart(p.r[i]) {
		return
	}
	e := p.scanName(i)
	name := string(p.r[i:e])
	i = p.skipS(e)
	q := p.at(i)
	if q != '"' && q != '\'' {
		return
	}
	end := i + 1
	for !p.eof(end) && p.r[end] != q {
		end++
	}
	if p.entities == nil {
		p.entities = map[string]string{}
	}
	if _, ok := p.entities[name]; !ok && !p.eof(end) {
		p.entities[name] = string(p.r[i+1 : end])
	}
}

// skipDecl skips to the '>' closing a declaration, outside quoted literals.
func (p *xparser) skipDecl(i int) (int, error) {
	var q rune
	for ; !p.eof(i); i++ {
		switch c := p.r[i]; {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
		case c == '>':
			return i + 1, nil
		}
	}
	return i, p.failEOF()
}

// Encoding names Python resolves (encodings.aliases, normalised) when expat does not know
// them: pyexpat then decodes byte by byte, so a UTF-8 or ASCII codec rejects every byte >= 0x80.
var (
	latin1Names = setOf("latin_1", "iso_8859_1", "8859", "cp819", "csisolatin1", "ibm819", "iso8859",
		"iso8859_1", "iso_8859_1_1987", "iso_ir_100", "l1", "latin", "latin1")
	byteRejectNames = setOf("utf_8", "utf8", "u8", "utf", "cp65001", "csutf8", "utf8_ucs2", "utf8_ucs4",
		"ascii", "646", "ansi_x3.4_1968", "ansi_x3.4_1986", "ansi_x3_4_1968", "cp367", "csascii",
		"ibm367", "iso646_us", "iso_646.irv_1991", "iso_ir_6", "us", "us_ascii")
)

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// normalizeEncoding is encodings.normalize_encoding plus lower(): runs of characters other
// than ASCII letters, digits and '.' become one '_', leading and trailing ones dropped.
func normalizeEncoding(name string) string {
	var b strings.Builder
	pending := false
	for _, c := range strings.ToLower(name) {
		if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '.' {
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(c)
			pending = false
		} else {
			pending = true
		}
	}
	return b.String()
}

// applyEncoding re-decodes the input for the encoding its XML declaration names. expat
// knows UTF-8, US-ASCII and ISO-8859-1; other names go through Python's codecs. Only
// Latin-1 and the UTF-8/ASCII aliases are modelled: any other name is "unknown encoding"
// (Python decodes any single-byte codec such as cp1252 there and raises LookupError for an
// unknown name). expat's own UTF-16 names conflict with the one-byte input: "incorrect" at the
// name. A UTF-8 byte order mark is read as UTF-8 before the switch, but positions after it are
// counted in the new encoding, so its three bytes become three columns.
func (p *xparser) applyEncoding(data []byte) error {
	start := p.bom
	if p.at(start) != '<' || p.at(start+1) != '?' || p.at(start+2) != 'x' || p.at(start+3) != 'm' ||
		p.at(start+4) != 'l' || !isS(p.at(start+5)) {
		return nil
	}
	if _, err := p.pi(start, true); err != nil {
		return nil // the parse reports it at the same place
	}
	raw := strings.ToLower(p.encoding)
	p.encoding = ""
	norm := normalizeEncoding(raw)
	switch {
	case raw == "" || raw == "utf-8":
		return nil
	case raw == "utf-16" || raw == "utf-16be" || raw == "utf-16le":
		return p.fail(p.encAt, errIncorrect)
	case raw == "us-ascii" || raw == "iso-8859-1" || latin1Names[norm] || byteRejectNames[norm]:
		latin1 := raw == "iso-8859-1" || latin1Names[norm]
		p.r = p.r[:0]
		for _, b := range data {
			c := rune(b)
			if b >= 0x80 && !latin1 {
				c = bad
			}
			p.r = append(p.r, c)
		}
		p.n = len(p.r)
		if p.bom > 0 {
			p.bom = 3 // EF BB BF, now one rune per byte: the prolog skips them unread
		}
		return nil
	}
	return p.fail(0, errEncoding)
}
