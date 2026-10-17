package bogwrite

import (
	"math/big"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// uniqueName is unique_name, the port of ObjectUtil.generateUniqueSlotName: Foo -> Foo1,
// Foo9 -> Foo10, Foo009 -> Foo0010.
func uniqueName(base string, existing map[string]bool) (string, error) {
	if !existing[base] {
		return base, nil
	}
	stem, num := base, big.NewInt(1)
	if pre, digits, ok := splitTrailingDigits(base); ok {
		n, err := pycompat.ParseBigInt(string(digits)) // int() of Unicode decimal digits
		if err != nil {
			return "", err
		}
		num = n.Add(n, big.NewInt(1))
		stem = pre
		if pre != "" {
			for _, ch := range digits[:len(digits)-1] {
				if ch != '0' {
					break
				}
				stem += "0"
			}
		}
	}
	for i := 0; i <= 100000; i++ {
		cand := stem + new(big.Int).Add(num, big.NewInt(int64(i))).String()
		if !existing[cand] {
			return cand, nil
		}
	}
	return "", raise("ValueError", "no unique name for %s", pyjson.StrRepr(base))
}

// splitTrailingDigits is re.match(r"^(.*?)(\d+)$", s): the shortest prefix before a run of
// Unicode decimal digits ending the text (or followed by one final newline, as $ allows).
func splitTrailingDigits(s string) (string, []rune, bool) {
	rs := pyjson.Runes(strings.TrimSuffix(s, "\n"))
	end := len(rs)
	i := end
	for i > 0 && pycompat.IsDecimal(rs[i-1]) {
		i--
	}
	if i == end || strings.ContainsRune(string(rs[:i]), '\n') {
		return "", nil, false
	}
	return pyjson.WTF8(rs[:i]), rs[i:end], true
}

// nameOK is _NAME_OK: ^[A-Za-z_][A-Za-z0-9_]*$ ($ also before one final newline).
func nameOK(s string) bool { return asciiName(s, "") }

// slotNameOK is _slot_name_ok: ^[A-Za-z_$][A-Za-z0-9_$]*$.
func slotNameOK(s string) bool { return s != "" && asciiName(s, "$") }

func asciiName(s, extra string) bool {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || strings.IndexByte(extra, c) >= 0 ||
			i > 0 && '0' <= c && c <= '9'
		if !ok {
			return false
		}
	}
	return true
}

// quoteattr is xml.sax.saxutils.quoteattr.
func quoteattr(s string) string {
	s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\n", "&#10;", "\r", "&#13;", "\t", "&#9;").Replace(s)
	if strings.Contains(s, `"`) {
		if strings.Contains(s, "'") {
			return `"` + strings.ReplaceAll(s, `"`, "&quot;") + `"`
		}
		return "'" + s + "'"
	}
	return `"` + s + `"`
}

// attr is ' k=<quoteattr(s)>'.
func attr(k, s string) string { return " " + k + "=" + quoteattr(s) }

// attrOf is attr for a plan value: quoteattr calls str.replace, so a non-str raises.
func attrOf(k string, v pyjson.Value) (string, error) {
	s, err := asStr(v, "replace")
	if err != nil {
		return "", err
	}
	return attr(k, s), nil
}

// surrogateError is the UnicodeEncodeError of str.encode("utf-8") on a lone surrogate.
func surrogateError(s string) error {
	rs := pyjson.Runes(s)
	isSurrogate := func(r rune) bool { return 0xD800 <= r && r <= 0xDFFF }
	for pos, r := range rs {
		if !isSurrogate(r) {
			continue
		}
		end := pos
		for end+1 < len(rs) && isSurrogate(rs[end+1]) {
			end++
		}
		if end == pos {
			return raise("UnicodeEncodeError", "'utf-8' codec can't encode character '\\u%04x' in position %d: surrogates not allowed", r, pos)
		}
		return raise("UnicodeEncodeError", "'utf-8' codec can't encode characters in position %d-%d: surrogates not allowed", pos, end)
	}
	return nil
}
