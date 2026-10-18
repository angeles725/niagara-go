package license

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Site is one textual getLicenseManager() occurrence of a candidate file
// (a _scan_file site record): the features it leads to, or why none was found.
type Site struct {
	Path, Module, Class string
	Line                int
	Features            []string // sorted; empty when Reason is set
	Reason              string   // "" when the site leads to a feature
}

// classifySites is the getLicenseManager() half of _scan_file.
func classifySites(text, rel, module, cls string, comments []Span, calls []Call) []Site {
	var sites []Site
	var blocks []Span
	blocksDone := false
	for i := 0; ; {
		start, end, ok := nextManager(text, i)
		if !ok {
			return sites
		}
		i = end
		site := Site{Path: rel, Line: lineOf(text, start), Module: module, Class: cls}
		if inSpans(start, comments) {
			site.Reason = "in a comment"
			sites = append(sites, site)
			continue
		}
		after := text[end:runesAfter(text, end, 200)]
		stmt := text[statementStart(text, start):start]
		var direct []Call
		for _, c := range calls {
			if c.pos > end && onlyDot(text[end:c.pos]) {
				direct = append(direct, c)
			}
		}
		_, notDecl := receiver(text, start)
		switch {
		case len(direct) > 0:
			settle(&site, direct)
		case !notDecl:
			site.Reason = "declaration of getLicenseManager()"
		case featuresCall(after):
			site.Reason = "getFeatures(): every licensed feature, none named"
		case instanceofCheck(after, stmt):
			site.Reason = "instanceof check, no feature named"
		default:
			v, ok := assignedVar(stmt)
			if !ok {
				site.Reason = "other use: " + collapseSpace(text[start:runesAfter(text, end, 40)])
				break
			}
			if !blocksDone {
				blocks, blocksDone = braceBlocks(text, comments), true
			}
			from, to := variableScope(text, blocks, v, start, stmt)
			var used []Call
			for _, c := range calls {
				if from <= c.pos && c.pos < to && c.Receiver[strings.LastIndex(c.Receiver, ".")+1:] == v {
					used = append(used, c)
				}
			}
			if len(used) > 0 {
				settle(&site, used)
			} else {
				site.Reason = fmt.Sprintf("assigned to '%s', never used for a feature call", v)
			}
		}
		sites = append(sites, site)
	}
}

// settle is _settle: the sorted features of calls, or the unresolvable argument.
func settle(site *Site, calls []Call) {
	seen := map[string]bool{}
	for _, c := range calls {
		if c.Resolved() && !seen[*c.Feature] {
			seen[*c.Feature] = true
			site.Features = append(site.Features, *c.Feature)
		}
	}
	slices.Sort(site.Features)
	if len(site.Features) == 0 {
		site.Reason = "feature argument not resolvable: " + calls[0].Args[1]
	}
}

// nextManager is the next _MANAGER_RE match (\bgetLicenseManager\s*\(\s*\))
// at or after byte i: its start and end.
func nextManager(text string, i int) (start, end int, ok bool) {
	const word = "getLicenseManager"
	for i <= len(text) {
		k := strings.Index(text[i:], word)
		if k < 0 {
			return 0, 0, false
		}
		start = i + k
		i = start + 1
		if wordBefore(text, start) {
			continue
		}
		p := skipSpace(text, start+len(word))
		if p >= len(text) || text[p] != '(' {
			continue
		}
		if p = skipSpace(text, p+1); p < len(text) && text[p] == ')' {
			return start, p + 1, true
		}
	}
	return 0, 0, false
}

// runesAfter is the byte offset n code points after byte i (clamped to the text).
func runesAfter(text string, i, n int) int {
	for ; n > 0 && i < len(text); n-- {
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return i
}

// statementStart is max(text.rfind(c, 0, pos) for c in ";{}") + 1.
func statementStart(text string, pos int) int {
	return strings.LastIndexAny(text[:pos], ";{}") + 1
}

// onlyDot is re.match(r'\s*\.\s*$', s).
func onlyDot(s string) bool {
	i := skipSpace(s, 0)
	return i < len(s) && s[i] == '.' && skipSpace(s, i+1) == len(s)
}

// featuresCall is re.match(r'\s*\.\s*getFeatures\s*\(', after).
func featuresCall(after string) bool {
	i := skipSpace(after, 0)
	if i >= len(after) || after[i] != '.' {
		return false
	}
	if i = lit(after, skipSpace(after, i+1), "getFeatures"); i < 0 {
		return false
	}
	i = skipSpace(after, i)
	return i < len(after) && after[i] == '('
}

// instanceofCheck is re.search(r'\binstanceof\b', <after up to the first ;{}>[:60])
// or re.search(r'\binstanceof\s*$', stmt).
func instanceofCheck(after, stmt string) bool {
	seg := after
	if k := strings.IndexAny(seg, ";{}"); k >= 0 {
		seg = seg[:k]
	}
	seg = seg[:runesAfter(seg, 0, 60)]
	if hasWord(seg, "instanceof") {
		return true
	}
	t := strings.TrimRightFunc(stmt, pycompat.IsSpace)
	return strings.HasSuffix(t, "instanceof") && !wordBefore(t, len(t)-len("instanceof"))
}

// hasWord is re.search(r'\b' + word + r'\b', s) for a word of word characters.
func hasWord(s, word string) bool {
	found := false
	occurrences(s, word, func(i int) bool {
		found = !wordBefore(s, i) && !wordAt(s, i+len(word))
		return found
	})
	return found
}

// assignedVar is the variable of
// re.search(r'([\w$]+)\s*=\s*(?:\([\w$.<>]+\)\s*)?(?:[\w$]+\s*\.\s*)?$', stmt).
// The tail after '=' cannot hold another '=', so only the last one can match;
// the leftmost start is the first character of the identifier run before it.
func assignedVar(stmt string) (string, bool) {
	e := strings.LastIndexByte(stmt, '=')
	if e < 0 {
		return "", false
	}
	j := skipSpace(stmt, e+1)
	if j < len(stmt) && stmt[j] == '(' {
		k := skipRun(stmt, j+1, castChar)
		if k == j+1 || k >= len(stmt) || stmt[k] != ')' {
			return "", false
		}
		j = skipSpace(stmt, k+1)
	}
	if j < len(stmt) {
		k := skipRun(stmt, j, isIdent)
		if k == j {
			return "", false
		}
		if k = skipSpace(stmt, k); k >= len(stmt) || stmt[k] != '.' {
			return "", false
		}
		if j = skipSpace(stmt, k+1); j != len(stmt) {
			return "", false
		}
	}
	end := e
	for end > 0 {
		r, n := utf8.DecodeLastRuneInString(stmt[:end])
		if !pycompat.IsSpace(r) {
			break
		}
		end -= n
	}
	start := end
	for start > 0 {
		r, n := utf8.DecodeLastRuneInString(stmt[:start])
		if !isIdent(r) {
			break
		}
		start -= n
	}
	if start == end {
		return "", false
	}
	return stmt[start:end], true
}

// castChar is [\w$.<>].
func castChar(r rune) bool { return isIdent(r) || r == '.' || r == '<' || r == '>' }

// typeChar is [\w$.<>\[\]].
func typeChar(r rune) bool { return castChar(r) || r == '[' || r == ']' }

// collapseSpace is re.sub(r'\s+', ' ', s).strip().
func collapseSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, pycompat.IsSpace), " ")
}

// braceBlocks is java_text.brace_blocks: every {...} block outside comments
// and string or char literals, an unclosed one ending at the end of the text.
func braceBlocks(text string, comments []Span) []Span {
	var blocks []Span
	var stack []int
	n, k := len(text), 0
	for i := 0; i < n; {
		for k < len(comments) && comments[k].End <= i {
			k++
		}
		if k < len(comments) && comments[k].Start <= i {
			i = comments[k].End
			continue
		}
		c := text[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && text[j] != c && text[j] != '\n' {
				if text[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			i = j + 1
			continue
		case c == '{':
			stack = append(stack, i)
		case c == '}' && len(stack) > 0:
			blocks = append(blocks, Span{stack[len(stack)-1], i})
			stack = stack[:len(stack)-1]
		}
		i++
	}
	for _, o := range stack {
		blocks = append(blocks, Span{o, n})
	}
	return blocks
}

// enclosing is java_text.enclosing: the blocks containing pos (open < pos <=
// close), innermost first.
func enclosing(blocks []Span, pos int) []Span {
	var out []Span
	for _, b := range blocks {
		if b.Start < pos && pos <= b.End {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b Span) int {
		if a.Start != b.Start {
			return b.Start - a.Start
		}
		return b.End - a.End
	})
	return out
}

// header is java_text.header: the text between the previous ';', '{' or '}'
// and the block's '{'.
func header(text string, open int) string { return text[statementStart(text, open):open] }

// isTypeBody is java_text.is_type_body: the body of a class, interface, enum,
// record or anonymous class.
func isTypeBody(text string, open int) bool {
	head := header(text, open)
	for _, kw := range []string{"class", "interface", "enum", "record"} {
		if hasWord(head, kw) {
			return true
		}
	}
	return anonymousClassHead(head)
}

// anonymousClassHead is re.search(r'\bnew\s+[\w$.<>\[\]]+\s*\([^;]*\)\s*$', head):
// a `new Type (` whose '(' follows the last ';' and precedes the closing ')'
// that ends the head.
func anonymousClassHead(head string) bool {
	t := strings.TrimRightFunc(head, pycompat.IsSpace)
	if !strings.HasSuffix(t, ")") {
		return false
	}
	closing, semi := len(t)-1, strings.LastIndexByte(t, ';')
	found := false
	occurrences(head, "new", func(k int) bool {
		if wordBefore(head, k) {
			return false
		}
		j := skipSpace(head, k+3)
		if j == k+3 {
			return false
		}
		e := skipRun(head, j, typeChar)
		if e == j {
			return false
		}
		p := skipSpace(head, e)
		found = p < closing && head[p] == '(' && p > semi
		return found
	})
	return found
}

// variableScope is _variable_scope: the [start, end) range where the variable
// name assigned at pos by the statement stmt is the same variable: a local
// from the assignment to the end of its declaring block, a field its whole
// class body.
func variableScope(text string, blocks []Span, name string, pos int, stmt string) (int, int) {
	thisField := thisFieldAssign(stmt, name)
	encl := enclosing(blocks, pos)
	var declaring []Span
	if declaredFinally(stmt, name) || thisField {
		declaring = encl[:min(1, len(encl))]
	} else {
		for _, b := range encl {
			if isTypeBody(text, b.Start) || declares(header(text, b.Start)+text[b.Start:pos], name) >= 0 {
				declaring = []Span{b}
				break
			}
		}
	}
	if len(declaring) == 0 {
		return pos, len(text)
	}
	d := declaring[0]
	if thisField || isTypeBody(text, d.Start) {
		body := d
		for _, b := range encl {
			if isTypeBody(text, b.Start) {
				body = b
				break
			}
		}
		return body.Start, body.End
	}
	return pos, d.End
}

// declares finds decl = r'[\w$>\]]\s+' + name + r'\s*[;=,)]' in s: the offset
// of the terminating character of the first match, or -1.
func declares(s, name string) int {
	return declaresFrom(s, name, func(int) bool { return true })
}

// declaredFinally is re.search(decl + r'[^=]*$', stmt): a declaration of name
// with no '=' after its terminating character.
func declaredFinally(stmt, name string) bool {
	return declaresFrom(stmt, name, func(t int) bool { return !strings.Contains(stmt[t+1:], "=") }) >= 0
}

// declaresFrom is declares restricted to matches whose terminator offset
// satisfies ok.
func declaresFrom(s, name string, ok func(t int) bool) int {
	at := -1
	occurrences(s, name, func(k int) bool {
		ws := k
		for ws > 0 {
			r, n := utf8.DecodeLastRuneInString(s[:ws])
			if !pycompat.IsSpace(r) {
				break
			}
			ws -= n
		}
		if ws == k || ws == 0 {
			return false
		}
		if r, _ := utf8.DecodeLastRuneInString(s[:ws]); !isIdent(r) && r != '>' && r != ']' {
			return false
		}
		t := skipSpace(s, k+len(name))
		if t >= len(s) || !strings.ContainsRune(";=,)", rune(s[t])) || !ok(t) {
			return false
		}
		at = t
		return true
	})
	return at
}

// thisFieldAssign is re.search(r'\bthis\s*\.\s*' + name + r'\s*=[^=]*$', stmt).
func thisFieldAssign(stmt, name string) bool {
	found := false
	occurrences(stmt, "this", func(k int) bool {
		if wordBefore(stmt, k) {
			return false
		}
		i := skipSpace(stmt, k+4)
		if i >= len(stmt) || stmt[i] != '.' {
			return false
		}
		if i = lit(stmt, skipSpace(stmt, i+1), name); i < 0 {
			return false
		}
		i = skipSpace(stmt, i)
		found = i < len(stmt) && stmt[i] == '=' && !strings.Contains(stmt[i+1:], "=")
		return found
	})
	return found
}
