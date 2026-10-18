package helpbuild

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The tokenizer of CPython 3.15's html.parser.HTMLParser, as html_text
// drives it: convert_charrefs=True, scripting=False, one feed() of the whole
// page, then close(). Only the callbacks html_text overrides are reported
// (start tag, start-end tag, end tag, data); comments, declarations,
// processing instructions and CDATA sections are consumed and dropped, as the
// base class's empty handlers drop them.
//
// One feed() of the whole page then close() parses exactly like a single
// goahead(end=1): every early stop of the feed() pass (an unterminated
// construct, a trailing "&" that could be a cut reference, script data
// without its end tag) leaves the parser state unchanged and close() resumes
// at the same position. The text after the last "<" is handed over whole in
// both cases, so the "&" look-back has no effect on what is reported.
//
// With convert_charrefs the "&" branches of goahead are unreachable (data
// runs to the next "<", or to the CDATA end tag), so data is unescaped with
// html.unescape (pycompat.HTMLUnescape) chunk by chunk, outside the raw-text
// elements (script, style, xmp, iframe, noembed, noframes, plaintext).
//
// Offsets are byte offsets into valid UTF-8: every delimiter the regexes test
// is ASCII and every class excludes ASCII bytes only, so a multi-byte rune is
// always consumed whole, as Python consumes one code point.

// htmlAttr is one (name, value) attribute pair; Null is Python's None value
// (an attribute without "=").
type htmlAttr struct {
	Name, Value string
	Null        bool
}

// htmlHandler receives the callbacks html_text overrides.
type htmlHandler interface {
	startTag(tag string, attrs []htmlAttr)
	startEndTag(tag string, attrs []htmlAttr)
	endTag(tag string)
	data(text string)
}

var (
	htmlCDATAContent  = map[string]bool{"script": true, "style": true, "xmp": true, "iframe": true, "noembed": true, "noframes": true}
	htmlRCDATAContent = map[string]bool{"textarea": true, "title": true}
)

type htmlTokenizer struct {
	raw       string
	h         htmlHandler
	cdataElem string // "" is None
	escapable bool
}

// parseHTML is HTMLParser(convert_charrefs=True).feed(page); close().
func parseHTML(page string, h htmlHandler) {
	p := &htmlTokenizer{raw: page, h: h, escapable: true}
	p.goahead()
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// isHTMLSpace is the class [\t\n\r\f ].
func isHTMLSpace(c byte) bool {
	return c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == ' '
}

// isTagNameChar is [^\t\n\r\f />]: a tag name after its first letter, and
// the first character of an attribute name.
func isTagNameChar(c byte) bool { return !isHTMLSpace(c) && c != '/' && c != '>' }

// isAttrNameChar is [^\t\n\r\f /=>].
func isAttrNameChar(c byte) bool { return isTagNameChar(c) && c != '=' }

// isAttrLookbehind is the (?<=['"\t\n\r\f /]) before an attribute name.
func isAttrLookbehind(c byte) bool { return c == '\'' || c == '"' || c == '/' || isHTMLSpace(c) }

func (p *htmlTokenizer) emit(s string) {
	if p.escapable {
		s = htmlUnescape(s)
	}
	p.h.data(s)
}

func (p *htmlTokenizer) goahead() {
	raw := p.raw
	n := len(raw)
	i := 0
	for i < n {
		var j int
		if p.cdataElem == "" {
			j = strings.IndexByte(raw[i:], '<')
			if j < 0 {
				j = n
			} else {
				j += i
			}
		} else {
			j = p.cdataEnd(i)
			if j < 0 {
				break
			}
		}
		if i < j {
			p.emit(raw[i:j])
		}
		i = j
		if i == n {
			break
		}
		var k int
		switch rest := raw[i:]; {
		case len(rest) > 1 && isASCIILetter(rest[1]):
			k = p.parseStarttag(i)
		case strings.HasPrefix(rest, "</"):
			k = p.parseEndtag(i)
		case strings.HasPrefix(rest, "<!--"):
			k = p.parseComment(i)
		case strings.HasPrefix(rest, "<?"):
			k = indexFrom(raw, ">", i+2, 1)
		case strings.HasPrefix(rest, "<!"):
			k = p.parseHTMLDeclaration(i)
		default:
			p.h.data("<")
			k = i + 1
		}
		if k < 0 {
			// end=1: an unterminated construct runs to the end of the
			// page; only a bare "</" at the very end is reported.
			if strings.HasPrefix(raw[i:], "</") && i+2 == n {
				p.h.data("</")
			}
			k = n
		}
		i = k
	}
	if i < n {
		p.emit(raw[i:])
	}
}

// indexFrom is the end of the first sep at or after from (plus extra-len(sep)
// bytes past its start: the value parse_pi and the declarations return), or
// -1.
func indexFrom(s, sep string, from, extra int) int {
	if from > len(s) {
		return -1
	}
	k := strings.Index(s[from:], sep)
	if k < 0 {
		return -1
	}
	return from + k + extra
}

// cdataEnd is interesting.search in CDATA mode: the next
// </elem(?=[\t\n\r\f />]) (re.IGNORECASE|re.ASCII, so only ASCII letters
// fold), or the end of the page for plaintext (\z); -1 when there is none.
func (p *htmlTokenizer) cdataEnd(i int) int {
	raw, elem := p.raw, p.cdataElem
	if elem == "plaintext" {
		return len(raw)
	}
	for {
		k := strings.Index(raw[i:], "</")
		if k < 0 {
			return -1
		}
		pos := i + k
		e := pos + 2 + len(elem)
		if e < len(raw) && asciiFoldEqual(raw[pos+2:e], elem) && isTagNameDelim(raw[e]) {
			return pos
		}
		i = pos + 1
	}
}

func isTagNameDelim(c byte) bool { return isHTMLSpace(c) || c == '/' || c == '>' }

// asciiFoldEqual compares s with the lower-case ASCII word w, folding ASCII
// letters only (re.ASCII: neither the Kelvin sign nor the long s folds).
func asciiFoldEqual(s, w string) bool {
	if len(s) != len(w) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != w[i] {
			return false
		}
	}
	return true
}

// attrValue matches the value indicator
// [\t\n\r\f ]*=[\t\n\r\f ]*('[^']*'|"[^"]*"|(?!['"])[^>\t\n\r\f ]*) at q, with
// sre's backtracking: a quote that is never closed makes the spaces before it
// give one back, so the value is the empty bare value before that quote.
func attrValue(s string, q int) (vs, ve int, ok bool) {
	n := len(s)
	a := q
	for a < n && isHTMLSpace(s[a]) {
		a++
	}
	if a >= n || s[a] != '=' {
		return 0, 0, false
	}
	a++
	b := a
	for b < n && isHTMLSpace(s[b]) {
		b++
	}
	if b < n && (s[b] == '\'' || s[b] == '"') {
		if c := strings.IndexByte(s[b+1:], s[b]); c >= 0 {
			return b, b + 2 + c, true
		}
		if b > a {
			return b - 1, b - 1, true
		}
		return 0, 0, false
	}
	e := b
	for e < n && !isHTMLSpace(s[e]) && s[e] != '>' {
		e++
	}
	return b, e, true
}

// locateTagEnd is locatetagend.match(s, p).end(), p at the tag name's
// letter. Every part after the name is optional, so the first path sre finds
// is the greedy one; only the value indicator backtracks (attrValue).
func locateTagEnd(s string, p int) int {
	n := len(s)
	q := p + 1
	for q < n && isTagNameChar(s[q]) {
		q++
	}
	for q < n && (isHTMLSpace(s[q]) || s[q] == '/') {
		q++
	}
	for q < n && isAttrLookbehind(s[q-1]) && isTagNameChar(s[q]) {
		q++
		for q < n && isAttrNameChar(s[q]) {
			q++
		}
		if _, ve, ok := attrValue(s, q); ok {
			q = ve
		}
		for q < n && (isHTMLSpace(s[q]) || s[q] == '/') {
			q++
		}
	}
	if q < n && s[q] == '>' {
		q++
	}
	return q
}

// skipTagSpace is (?:[\t\n\r\f ]|/(?!>))* from q.
func skipTagSpace(s string, q int) int {
	for q < len(s) && (isHTMLSpace(s[q]) || s[q] == '/' && (q+1 >= len(s) || s[q+1] != '>')) {
		q++
	}
	return q
}

// tagFind is tagfind_tolerant.match(s, p): the end of the match and group 1.
func tagFind(s string, p int) (end int, name string) {
	q := p + 1
	for q < len(s) && isTagNameChar(s[q]) {
		q++
	}
	return skipTagSpace(s, q), s[p:q]
}

// attrFind is attrfind_tolerant.match(s, k): the attribute (value as the
// regex captured it, quotes included) and the end of the match.
func attrFind(s string, k int) (a htmlAttr, end int, ok bool) {
	if k == 0 || k >= len(s) || !isAttrLookbehind(s[k-1]) || !isTagNameChar(s[k]) {
		return a, 0, false
	}
	q := k + 1
	for q < len(s) && isAttrNameChar(s[q]) {
		q++
	}
	a.Name = s[k:q]
	if vs, ve, has := attrValue(s, q); has {
		a.Value, q = s[vs:ve], ve
	} else {
		a.Null = true
	}
	return a, skipTagSpace(s, q), true
}

func (p *htmlTokenizer) parseStarttag(i int) int {
	raw := p.raw
	endpos := locateTagEnd(raw, i+1)
	if raw[endpos-1] != '>' {
		return -1
	}
	k, name := tagFind(raw, i+1)
	tag := pycompat.Lower(name)
	var attrs []htmlAttr
	for k < endpos {
		a, end, ok := attrFind(raw, k)
		if !ok {
			break
		}
		if !a.Null {
			v := a.Value
			if len(v) > 0 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
				if len(v) >= 2 {
					v = v[1 : len(v)-1]
				} else {
					v = ""
				}
			}
			if v != "" {
				v = unescapeAttrValue(v)
			}
			a.Value = v
		}
		a.Name = pycompat.Lower(a.Name)
		attrs = append(attrs, a)
		k = end
	}
	end := ""
	if k < endpos {
		end = pycompat.Strip(raw[k:endpos])
	}
	if end != ">" && end != "/>" {
		p.h.data(raw[i:endpos])
		return endpos
	}
	if strings.HasSuffix(end, "/>") {
		p.h.startEndTag(tag, attrs)
		return endpos
	}
	p.h.startTag(tag, attrs)
	switch {
	case htmlCDATAContent[tag] || tag == "plaintext":
		p.cdataElem, p.escapable = tag, false
	case htmlRCDATAContent[tag]:
		p.cdataElem, p.escapable = tag, true
	}
	return endpos
}

func (p *htmlTokenizer) parseEndtag(i int) int {
	raw := p.raw
	if strings.IndexByte(raw[i+2:], '>') < 0 {
		return -1
	}
	if !isASCIILetter(raw[i+2]) {
		if raw[i+2] == '>' { // </> is ignored
			return i + 3
		}
		return indexFrom(raw, ">", i+2, 1) // bogus comment
	}
	j := locateTagEnd(raw, i+2)
	if raw[j-1] != '>' {
		return -1
	}
	_, name := tagFind(raw, i+2)
	p.h.endTag(pycompat.Lower(name))
	p.cdataElem, p.escapable = "", true
	return j
}

// parseComment: an empty comment closes abruptly at ">" or "->", else at
// the first "-->" or "--!>".
func (p *htmlTokenizer) parseComment(i int) int {
	raw := p.raw
	s := i + 4
	switch {
	case strings.HasPrefix(raw[s:], ">"):
		return s + 1
	case strings.HasPrefix(raw[s:], "->"):
		return s + 2
	}
	for q := s; ; q++ {
		k := strings.Index(raw[q:], "--")
		if k < 0 {
			return -1
		}
		q += k
		switch {
		case strings.HasPrefix(raw[q+2:], ">"):
			return q + 3
		case strings.HasPrefix(raw[q+2:], "!>"):
			return q + 4
		}
	}
}

func (p *htmlTokenizer) parseHTMLDeclaration(i int) int {
	raw := p.raw
	switch {
	case strings.HasPrefix(raw[i:], "<![CDATA["):
		return indexFrom(raw, "]]>", i+9, 3)
	case i+9 <= len(raw) && asciiFoldEqual(raw[i:i+9], "<!doctype"):
		return indexFrom(raw, ">", i+9, 1)
	}
	return indexFrom(raw, ">", i+2, 1) // bogus comment
}
