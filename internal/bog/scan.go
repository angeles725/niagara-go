package bog

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// The BOG grammar of bognav/model.py (verbatim from the bog-audit.sh D10 engine), hand-compiled:
//
//	TAG_RE = <(/?)([A-Za-z]\w*)\b([^>]*?)(/?)>   applied with finditer to each splitlines() line
//	ga()   = \bNAME='([^']*)'  then  \bNAME="([^"]*)"   searched in the matched tag text
//	_ESC_RE = \$(?:u([0-9a-fA-F]{4})|([0-9a-fA-F]{2}))
//
// Python's \w and \b are Unicode-aware on str patterns; Go's regexp is ASCII-only, hence the
// scanner. A regex match cannot cross a splitlines() boundary, so the whole text is scanned and a
// tag simply never extends past one.

// tag is one TAG_RE match.
type tag struct {
	closing   bool
	name      string
	selfClose bool
	full      string // the whole match, attributes included
}

// tagScanner yields the TAG_RE matches of a text in document order.
type tagScanner struct {
	s   string
	pos int
}

func (sc *tagScanner) next() (tag, bool) {
	s := sc.s
	for {
		i := strings.IndexByte(s[sc.pos:], '<')
		if i < 0 {
			sc.pos = len(s)
			return tag{}, false
		}
		i += sc.pos
		if t, end, ok := matchTag(s, i); ok {
			sc.pos = end
			return t, true
		}
		sc.pos = i + 1
	}
}

// matchTag tries TAG_RE at s[i] == '<' and returns the match and the index after it.
func matchTag(s string, i int) (tag, int, bool) {
	j := i + 1
	closing := j < len(s) && s[j] == '/'
	if closing {
		j++
	}
	if j >= len(s) || !isASCIILetter(s[j]) {
		return tag{}, 0, false
	}
	k := j + 1
	for k < len(s) {
		r, size := rune(s[k]), 1
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(s[k:])
		}
		if !isWordRune(r) {
			break
		}
		k += size
	}
	// [^>]*? then (/?)> : the first '>' on the same line closes the tag.
	e := k
	for {
		if e >= len(s) || lineBreakAt(s, e) > 0 {
			return tag{}, 0, false
		}
		if s[e] == '>' {
			break
		}
		e++
	}
	return tag{
		closing:   closing,
		name:      s[j:k],
		selfClose: s[e-1] == '/' && e-1 >= k,
		full:      s[i : e+1],
	}, e + 1, true
}

func isASCIILetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// attrName holds the two needles ga() searches for an attribute: NAME=' and NAME=".
type attrName struct{ single, double string }

func newAttrName(name string) attrName { return attrName{name + "='", name + "=\""} }

var (
	attrN = newAttrName("n")
	attrH = newAttrName("h")
	attrT = newAttrName("t")
	attrV = newAttrName("v")
	attrM = newAttrName("m")
	attrF = newAttrName("f")
)

// attr is ga(): the value of an attribute in a tag's text, single-quoted form first.
func attr(full string, name attrName) (string, bool) {
	if v, ok := quotedAttr(full, name.single); ok {
		return v, true
	}
	return quotedAttr(full, name.double)
}

// quotedAttr is \bNEEDLE([^q]*)q where needle ends with the quote q.
func quotedAttr(full, needle string) (string, bool) {
	q := needle[len(needle)-1]
	for from := 0; ; {
		i := strings.Index(full[from:], needle)
		if i < 0 {
			return "", false
		}
		i += from
		start := i + len(needle)
		if wordBoundaryBefore(full, i) {
			end := strings.IndexByte(full[start:], q)
			if end < 0 {
				return "", false // no closing quote after this or any later opening quote
			}
			return full[start : start+end], true
		}
		from = i + 1
	}
}

// wordBoundaryBefore is \b before a word character at s[i]: s[i-1] is not a word character.
func wordBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !isWordRune(r)
}

// UnescapeName decodes a Niagara SlotPath-escaped name: "$20" -> " ", "$u00e9" -> "é".
func UnescapeName(name string) string {
	i := strings.IndexByte(name, '$')
	if i < 0 {
		return name
	}
	var sb strings.Builder
	sb.Grow(len(name))
	sb.WriteString(name[:i])
	for i < len(name) {
		if name[i] != '$' {
			sb.WriteByte(name[i])
			i++
			continue
		}
		if i+6 <= len(name) && name[i+1] == 'u' && isHex(name[i+2:i+6]) {
			v, _ := strconv.ParseUint(name[i+2:i+6], 16, 32)
			sb.WriteRune(rune(v))
			i += 6
			continue
		}
		if i+3 <= len(name) && isHex(name[i+1:i+3]) {
			v, _ := strconv.ParseUint(name[i+1:i+3], 16, 32)
			sb.WriteRune(rune(v))
			i += 3
			continue
		}
		sb.WriteByte('$')
		i++
	}
	return sb.String()
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}
