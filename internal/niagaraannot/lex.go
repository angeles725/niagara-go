package niagaraannot

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokKind int

const (
	tIdent  tokKind = iota + 1 // identifier or keyword
	tNumber                    // numeric literal as written (0x10, 1L)
	tString                    // string or text-block literal; text is the decoded value
	tChar                      // char literal as written
	tPunct                     // one punctuation character
)

// token is one Java token: its kind, its text and where it is in the source.
type token struct {
	kind     tokKind
	text     string
	pos, end int // byte offsets of the token in the source
	line     int // 1-based line of pos
}

// lex splits Java source into tokens, dropping whitespace and comments. An unterminated
// comment or literal is an error.
func lex(src string) ([]token, error) {
	var toks []token
	line := 1
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated comment", line)
			}
			body := src[i : i+2+end+2]
			line += strings.Count(body, "\n")
			i += len(body)
		case strings.HasPrefix(src[i:], `"""`):
			end := strings.Index(src[i+3:], `"""`)
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated text block", line)
			}
			body := src[i : i+3+end+3]
			toks = append(toks, token{tString, textBlock(src[i+3 : i+3+end]), i, i + len(body), line})
			line += strings.Count(body, "\n")
			i += len(body)
		case c == '"' || c == '\'':
			j, err := quoted(src, i)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", line, err)
			}
			if c == '"' {
				toks = append(toks, token{tString, unescape(src[i+1 : j-1]), i, j, line})
			} else {
				toks = append(toks, token{tChar, src[i:j], i, j, line})
			}
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < len(src) && (isIdentByte(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, token{tNumber, src[i:j], i, j, line})
			i = j
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == '_' || r == '$' || unicode.IsLetter(r) {
				j := i + size
				for j < len(src) {
					r, n := utf8.DecodeRuneInString(src[j:])
					if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
						break
					}
					j += n
				}
				toks = append(toks, token{tIdent, src[i:j], i, j, line})
				i = j
				continue
			}
			toks = append(toks, token{tPunct, src[i : i+size], i, i + size, line})
			i += size
		}
	}
	return toks, nil
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// quoted returns the end offset (past the closing quote) of the literal opening at src[i].
func quoted(src string, i int) (int, error) {
	q := src[i]
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '\n':
			return 0, fmt.Errorf("unterminated %c literal", q)
		case q:
			return j + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated %c literal", q)
}

// unescape decodes the Java escapes of a string literal body; an unknown escape keeps its
// character.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 's':
			b.WriteByte(' ')
		case 'u':
			for i+1 < len(s) && s[i+1] == 'u' {
				i++
			}
			if i+4 < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 4
					continue
				}
			}
			b.WriteString(`\u`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// textBlock is the value of a text block: the content after its opening line, escapes decoded.
// The incidental indentation is not stripped (no annotation the course reads uses one).
func textBlock(body string) string {
	if _, rest, ok := strings.Cut(body, "\n"); ok {
		body = rest
	}
	return unescape(body)
}
