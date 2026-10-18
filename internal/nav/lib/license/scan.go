package license

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Call is one licensing call (a _scan_file call record).
type Call struct {
	Path, Module, Class string
	Line                int      // 1-based line of the method name
	Form                string   // checkFeature or getFeature
	Receiver            string   // getLicenseManager(), a dotted receiver or this
	Vendor, Feature     *string  // resolved arguments; nil when unresolvable
	Args                []string // the two call arguments as written

	pos int // byte offset of the method name (site classification only)
}

// Resolved is `if c["feature"]`: the feature argument resolved to a non-empty name.
func (c Call) Resolved() bool { return c.Feature != nil && *c.Feature != "" }

// keywordsBeforeCall is _KEYWORDS_BEFORE_CALL.
var keywordsBeforeCall = map[string]bool{"return": true, "throw": true, "else": true, "case": true,
	"yield": true, "assert": true}

// nextCall is the next _CALL_RE match at or after byte i:
// \b(checkFeature|getFeature)\s*\( -> its start, method and '(' offset.
func nextCall(text string, i int) (start int, method string, paren int, ok bool) {
	for i < len(text) {
		best, form := -1, ""
		for _, m := range []string{"checkFeature", "getFeature"} {
			if k := strings.Index(text[i:], m); k >= 0 && (best < 0 || i+k < best) {
				best, form = i+k, m
			}
		}
		if best < 0 {
			return 0, "", 0, false
		}
		if !wordBefore(text, best) {
			if p := skipSpace(text, best+len(form)); p < len(text) && text[p] == '(' {
				return best, form, p, true
			}
		}
		i = best + 1
	}
	return 0, "", 0, false
}

// window is text[max(0, pos-300):pos] in code points.
func window(text string, pos int) string {
	start := pos
	for k := 0; k < 300 && start > 0; k++ {
		_, n := utf8.DecodeLastRuneInString(text[:start])
		start -= n
	}
	return text[start:pos]
}

func rstrip(s string) string { return strings.TrimRightFunc(s, pycompat.IsSpace) }

// receiver is _receiver: "getLicenseManager()", a dotted receiver ("?" when
// none), "this", or ok=false for a declaration (`Feature getFeature(...)`).
func receiver(text string, callStart int) (string, bool) {
	before := rstrip(window(text, callStart))
	if strings.HasSuffix(before, ".") {
		before = rstrip(before[:len(before)-1])
		if endsWithManagerCall(before) {
			return "getLicenseManager()", true
		}
		// ([\w$][\w$.\[\]]*)$: the first [\w$] of the longest suffix of [\w$.\[\]].
		s := len(before)
		for s > 0 {
			r, n := utf8.DecodeLastRuneInString(before[:s])
			if !(isIdent(r) || r == '.' || r == '[' || r == ']') {
				break
			}
			s -= n
		}
		for p := s; p < len(before); {
			r, n := utf8.DecodeRuneInString(before[p:])
			if isIdent(r) {
				return before[p:], true
			}
			p += n
		}
		return "?", true
	}
	s := len(before)
	for s > 0 {
		r, n := utf8.DecodeLastRuneInString(before[:s])
		if !isIdent(r) {
			break
		}
		s -= n
	}
	if s < len(before) && !keywordsBeforeCall[before[s:]] {
		return "", false
	}
	return "this", true
}

// endsWithManagerCall is re.search(r'getLicenseManager\s*\(\s*\)$', s).
func endsWithManagerCall(s string) bool {
	if !strings.HasSuffix(s, ")") {
		return false
	}
	s = rstrip(s[:len(s)-1])
	if !strings.HasSuffix(s, "(") {
		return false
	}
	return strings.HasSuffix(rstrip(s[:len(s)-1]), "getLicenseManager")
}

// isLicenseCall is _is_license_call: false for DOM/XML getFeature(feature, version).
func isLicenseCall(method, recv, text, rel string) bool {
	if method == "checkFeature" || recv == "getLicenseManager()" {
		return true
	}
	v := recv[strings.LastIndex(recv, ".")+1:]
	if strings.Contains(textfold.Lower(recv), "licen") || strings.HasSuffix(recv, "INSTANCE") {
		return true
	}
	if recv == "this" {
		return strings.Contains(path.Base(rel), "LicenseManager")
	}
	return typedManagerVar(text, v) || assignedManagerVar(text, v)
}

// typedManagerVar is re.search(r'\b\w*LicenseManager\s*\)?\s*' + escape(v) + r'\b', text):
// a variable declared or cast as a *LicenseManager. The \b\w* prefix always
// matches (from the start of the word run ending at "LicenseManager").
func typedManagerVar(text, v string) bool {
	found := false
	occurrences(text, "LicenseManager", func(q int) bool {
		i := skipSpace(text, q+len("LicenseManager"))
		if i < len(text) && text[i] == ')' {
			i = skipSpace(text, i+1)
		}
		if e := lit(text, i, v); e >= 0 && boundary(text, e) {
			found = true
		}
		return found
	})
	return found
}

// assignedManagerVar is re.search(r'\b' + escape(v) + r'\s*=\s*[^;]*getLicenseManager\s*\(', text).
func assignedManagerVar(text, v string) bool {
	found := false
	occurrences(text, v, func(p int) bool {
		if !boundary(text, p) {
			return false
		}
		eq := skipSpace(text, p+len(v))
		if eq >= len(text) || text[eq] != '=' {
			return false
		}
		end := len(text)
		if k := strings.IndexByte(text[eq+1:], ';'); k >= 0 {
			end = eq + 1 + k
		}
		region := text[:end]
		occurrences(region[eq+1:], "getLicenseManager", func(k int) bool {
			i := skipSpace(region, eq+1+k+len("getLicenseManager"))
			found = i < len(region) && region[i] == '('
			return found
		})
		return found
	})
	return found
}

// scanFile is _scan_file: the licensing calls of one source file and the
// classification of its getLicenseManager() sites.
func scanFile(text, rel, module, cls string, res *resolver) ([]Call, []Site) {
	comments := CommentSpans(text)
	var calls []Call
	for i := 0; ; {
		start, method, paren, ok := nextCall(text, i)
		if !ok {
			break
		}
		i = paren + 1
		if inSpans(start, comments) {
			continue
		}
		recv, ok := receiver(text, start)
		args, balanced := callArgs(text, paren)
		if !ok || !balanced || len(args) != 2 || !isLicenseCall(method, recv, text, rel) {
			continue
		}
		calls = append(calls, Call{Path: rel, Line: lineOf(text, start), Module: module, Class: cls,
			Form: method, Receiver: recv, Vendor: res.resolve(args[0], rel),
			Feature: res.resolve(args[1], rel), Args: args, pos: start})
	}
	return calls, classifySites(text, rel, module, cls, comments, calls)
}

// lineOf is java_text.line_of: the 1-based line of byte pos.
func lineOf(text string, pos int) int { return strings.Count(text[:pos], "\n") + 1 }
