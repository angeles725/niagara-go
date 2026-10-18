package pyhtml

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/textfold"
)

// The tokenizer of CPython 3.15's html.parser.HTMLParser with
// convert_charrefs=True, scripting=False, as the niagara-help tools drive it:
// one feed() of the whole page, then close() (Parse). Only the callbacks the
// tools override are reported (start tag, start-end tag, end tag, data);
// comments, declarations, processing instructions and CDATA sections are
// consumed and dropped, as the base class's empty handlers drop them.
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
// html.unescape (Unescape) chunk by chunk, outside the raw-text
// elements (script, style, xmp, iframe, noembed, noframes, plaintext).
//
// Offsets are byte offsets into valid UTF-8: every delimiter the regexes test
// is ASCII and every class excludes ASCII bytes only, so a multi-byte rune is
// always consumed whole, as Python consumes one code point.

// Attr is one (name, value) attribute pair; Null is Python's None value
// (an attribute without "=").
type Attr struct {
	Name, Value string
	Null        bool
}

// Handler receives the callbacks: handle_starttag, handle_startendtag,
// handle_endtag and handle_data (tag and attribute names lower-cased, data
// and attribute values unescaped as html.parser does).
type Handler interface {
	StartTag(tag string, attrs []Attr)
	StartEndTag(tag string, attrs []Attr)
	EndTag(tag string)
	Data(text string)
}

var (
	htmlCDATAContent  = map[string]bool{"script": true, "style": true, "xmp": true, "iframe": true, "noembed": true, "noframes": true}
	htmlRCDATAContent = map[string]bool{"textarea": true, "title": true}
)

type tokenizer struct {
	raw       string
	h         Handler
	cdataElem string // "" is None
	escapable bool
	end       bool // goahead(end): close() was called
}

// Parse is HTMLParser(convert_charrefs=True).feed(page); close(), the
// callbacks going to h.
func Parse(page string, h Handler) {
	p := &tokenizer{raw: page, h: h, escapable: true, end: true}
	p.goahead()
}

// Feed is HTMLParser(convert_charrefs=True).feed(page) without close(): one
// goahead(end=0) (feed() parses at once: the first call's threshold is one
// character). What would wait for more input is never reported: an
// unterminated construct and a lone "<" at the end with all that follows,
// raw text without its end tag, and the data after the last "<" when an "&"
// in its last 34 code points has no [\t\n\r\f ;] after it (heldCharref).
func Feed(page string, h Handler) {
	p := &tokenizer{raw: page, h: h, escapable: true}
	p.goahead()
}

// heldCharref is the look-back of goahead(end=0) when no "<" follows i:
// rawdata.rfind('&', max(i, n-34)) found an "&" that no [\t\n\r\f ;]
// follows, so the data could end in a cut reference and waits. n-34 counts
// code points, as Python's str indices do.
func heldCharref(raw string, i int) bool {
	lo := len(raw)
	for c := 0; c < 34 && lo > 0; c++ {
		_, size := utf8.DecodeLastRuneInString(raw[:lo])
		lo -= size
	}
	lo = max(lo, i)
	amp := strings.LastIndexByte(raw[lo:], '&')
	return amp >= 0 && !strings.ContainsAny(raw[lo+amp:], "\t\n\r\f ;")
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

func (p *tokenizer) emit(s string) {
	if p.escapable {
		s = Unescape(s)
	}
	p.h.Data(s)
}

func (p *tokenizer) goahead() {
	raw := p.raw
	n := len(raw)
	i := 0
	for i < n {
		var j int
		if p.cdataElem == "" {
			j = strings.IndexByte(raw[i:], '<')
			if j < 0 {
				if !p.end && heldCharref(raw, i) {
					break
				}
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
		case i+1 < n || p.end:
			p.h.Data("<")
			k = i + 1
		default: // a lone "<" at the end of a feed() waits for more
			k = -1
		}
		if k < 0 {
			if !p.end {
				break
			}
			// end=1: an unterminated construct runs to the end of the
			// page; only a bare "</" at the very end is reported.
			if strings.HasPrefix(raw[i:], "</") && i+2 == n {
				p.h.Data("</")
			}
			k = n
		}
		i = k
	}
	if p.end && i < n {
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
func (p *tokenizer) cdataEnd(i int) int {
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
func attrFind(s string, k int) (a Attr, end int, ok bool) {
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

func (p *tokenizer) parseStarttag(i int) int {
	raw := p.raw
	endpos := locateTagEnd(raw, i+1)
	if raw[endpos-1] != '>' {
		return -1
	}
	k, name := tagFind(raw, i+1)
	tag := textfold.Lower(name)
	var attrs []Attr
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
		a.Name = textfold.Lower(a.Name)
		attrs = append(attrs, a)
		k = end
	}
	end := ""
	if k < endpos {
		end = strings.TrimFunc(raw[k:endpos], textfold.IsSpace) // str.strip()
	}
	if end != ">" && end != "/>" {
		p.h.Data(raw[i:endpos])
		return endpos
	}
	if strings.HasSuffix(end, "/>") {
		p.h.StartEndTag(tag, attrs)
		return endpos
	}
	p.h.StartTag(tag, attrs)
	switch {
	case htmlCDATAContent[tag] || tag == "plaintext":
		p.cdataElem, p.escapable = tag, false
	case htmlRCDATAContent[tag]:
		p.cdataElem, p.escapable = tag, true
	}
	return endpos
}

func (p *tokenizer) parseEndtag(i int) int {
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
	p.h.EndTag(textfold.Lower(name))
	p.cdataElem, p.escapable = "", true
	return j
}

// parseComment: an empty comment closes abruptly at ">" or "->", else at
// the first "-->" or "--!>".
func (p *tokenizer) parseComment(i int) int {
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

func (p *tokenizer) parseHTMLDeclaration(i int) int {
	raw := p.raw
	switch {
	case strings.HasPrefix(raw[i:], "<![CDATA["):
		return indexFrom(raw, "]]>", i+9, 3)
	case i+9 <= len(raw) && asciiFoldEqual(raw[i:i+9], "<!doctype"):
		return indexFrom(raw, ">", i+9, 1)
	}
	return indexFrom(raw, ">", i+2, 1) // bogus comment
}
