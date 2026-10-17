package pxrender

import (
	"errors"
	"html"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// isPySpace is Python str.isspace: Go's unicode.IsSpace plus the four ASCII separators 0x1c-0x1f.
func isPySpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

func pySplit(s string) []string { return strings.FieldsFunc(s, isPySpace) }

var pyFloatRE = regexp.MustCompile(`^[+-]?(?:[0-9](?:_?[0-9])*(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)` +
	`(?:[eE][+-]?[0-9](?:_?[0-9])*)?$`)

// pyFloat parses like Python float(str): surrounding whitespace, underscores between digits,
// inf/infinity/nan in any case; no hex.
func pyFloat(s string) (float64, bool) {
	s = pyStrip(s)
	body, neg := s, false
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body, neg = body[1:], body[0] == '-'
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if neg {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	if !pyFloatRE.MatchString(s) {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return v, true
}

// pyInt is Python int(float) of a finite value as decimal text (no int64 overflow).
func pyInt(v float64) (string, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "", false
	}
	i, _ := big.NewFloat(v).Int(nil)
	return i.String(), true
}

// pyG is Python "%g" % v: 6 significant digits, trailing zeros dropped, e+XX exponents.
func pyG(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	return strconv.FormatFloat(v, 'g', 6, 64)
}

// htmlEscape is Python html.escape(s, quote=True).
var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;")

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }

var charrefRE = regexp.MustCompile(`&(#[0-9]+;?|#[xX][0-9a-fA-F]+;?|[^\t\n\f <&#;]{1,32};?)`)

// cp1252 maps the numeric references 0x80-0x9f the way Python html._invalid_charrefs does.
var cp1252 = [32]rune{0x20ac, 0x81, 0x201a, 0x192, 0x201e, 0x2026, 0x2020, 0x2021, 0x2c6, 0x2030, 0x160,
	0x2039, 0x152, 0x8d, 0x17d, 0x8f, 0x90, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x2dc,
	0x2122, 0x161, 0x203a, 0x153, 0x9d, 0x17e, 0x178}

// invalidCodepoint is Python html._invalid_codepoints: references to these are dropped.
func invalidCodepoint(n int64) bool {
	return n >= 0x1 && n <= 0x8 || n == 0xb || n >= 0xe && n <= 0x1f || n >= 0x7f && n <= 0x9f ||
		n >= 0xfdd0 && n <= 0xfdef || n&0xfffe == 0xfffe && n <= 0x10ffff
}

// pyUnescape is Python html.unescape. Named references go through Go's HTML5 table, which
// resolves exactly the token Python's regex matched; numeric ones follow Python's rules.
func pyUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return charrefRE.ReplaceAllStringFunc(s, func(m string) string {
		ref := m[1:]
		if ref[0] != '#' {
			return html.UnescapeString(m)
		}
		digits, base := strings.TrimSuffix(ref[1:], ";"), 10
		if digits[0] == 'x' || digits[0] == 'X' {
			digits, base = digits[1:], 16
		}
		n, err := strconv.ParseInt(digits, base, 64)
		if err != nil || n > 0x10ffff {
			return "�"
		}
		switch {
		case n == 0:
			return "�"
		case n == 0x0d:
			return "\r"
		case n >= 0x80 && n <= 0x9f:
			return string(cp1252[n-0x80])
		case n >= 0xd800 && n <= 0xdfff:
			return "�"
		case invalidCodepoint(n):
			return ""
		}
		return string(rune(n))
	})
}

// pyBasename and pySplitextRoot are os.path.basename and os.path.splitext(...)[0].
func pyBasename(p string) string { return p[strings.LastIndex(p, "/")+1:] }

func pySplitextRoot(p string) string {
	sep := strings.LastIndex(p, "/")
	dot := strings.LastIndex(p, ".")
	if dot > sep {
		for i := sep + 1; i < dot; i++ {
			if p[i] != '.' {
				return p[:dot]
			}
		}
	}
	return p
}

// pyJoin is posixpath.join: an absolute part restarts the path; nothing is normalised.
func pyJoin(parts ...string) string {
	out := ""
	for i, p := range parts {
		switch {
		case i == 0 || strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}
