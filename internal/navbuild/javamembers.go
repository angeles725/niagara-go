package navbuild

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The Java member scanner of build_method_index.py (count_braces,
// split_params, extract_param_type, try_parse_method and
// parse_methods_from_content), which the call graph builder reuses for its
// method zones. It is a line scanner, not a parser: every quirk below is what
// the indexes hold today.

// methodModifiers is MODIFIERS of build_method_index.py.
var methodModifiers = setOf("public private protected static abstract final synchronized native strictfp default")

// notMethodNames is NOT_METHOD_NAMES: keywords that look like a call.
var notMethodNames = setOf(`if else while for do switch try catch finally
	return throw new super this case import package
	class interface enum assert break continue goto
	instanceof var record sealed permits yield null
	true false void`)

func setOf(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

// countBraces is count_braces: the { and } of line outside "..." and '...'
// literals (a backslash escapes the next character inside one) and before a
// // comment. Only ASCII characters matter, so it walks bytes.
func countBraces(line string) (opens, closes int) {
	inStr, inChar, escaped := false, false, false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case escaped:
			escaped = false
		case ch == '\\' && (inStr || inChar):
			escaped = true
		case ch == '"' && !inChar:
			inStr = !inStr
		case ch == '\'' && !inStr:
			inChar = !inChar
		case inStr || inChar:
		case ch == '/' && i+1 < len(line) && line[i+1] == '/':
			return opens, closes
		case ch == '{':
			opens++
		case ch == '}':
			closes++
		}
	}
	return opens, closes
}

// splitParams is split_params: params split at the commas outside <...>,
// each stripped, the empty ones dropped.
func splitParams(params string) []string {
	if pycompat.Strip(params) == "" {
		return nil
	}
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case '<':
			depth++
		case '>':
			depth = max(0, depth-1)
		case ',':
			if depth == 0 {
				parts = append(parts, params[start:i])
				start = i + 1
			}
		}
	}
	if start < len(params) { // Python's `if current:` (the text after the last comma)
		parts = append(parts, params[start:])
	}
	var out []string
	for _, p := range parts {
		if p = pycompat.Strip(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// extractParamType is extract_param_type: the type of "Type name", after
// leading annotations and "final ". Where Python raises IndexError (an
// annotation with arguments and nothing after it, "@A(x y)"), it is "".
func extractParamType(param string) string {
	param = pycompat.Strip(param)
	if param == "" {
		return ""
	}
	for strings.HasPrefix(param, "@") {
		paren := strings.IndexByte(param, '(')
		space := strings.IndexByte(param, ' ')
		if space < 0 {
			return param
		}
		next := pycompat.Strip(param[space+1:])
		if 0 < paren && paren < space {
			depth := 0
			for k := 0; k < len(param); k++ {
				if param[k] == '(' {
					depth++
				} else if param[k] == ')' {
					depth--
					if depth == 0 {
						next = pycompat.Strip(param[k+1:])
						break
					}
				}
			}
		}
		param = next
	}
	if strings.HasPrefix(param, "final ") {
		param = pycompat.Strip(param[len("final "):])
	}
	// param.rsplit(None, 1)[0]
	if i := strings.LastIndexFunc(param, textfold.IsSpace); i >= 0 {
		return strings.TrimRightFunc(param[:i], textfold.IsSpace)
	}
	return param
}

// javaMethod is one declaration found by parseMethods.
type javaMethod struct {
	Name, IndexedName, Class, ReturnType, Signature string
	Modifiers, Params                               []string
	Line                                            int // 1-based
}

// tryParseMethod is try_parse_method on the stripped line i of lines: the
// declaration and how many continuation lines its parameters took, or ok
// false.
func tryParseMethod(stripped string, lines []string, i int) (m javaMethod, extra int, ok bool) {
	paren := strings.IndexByte(stripped, '(')
	if paren < 0 {
		return m, 0, false
	}
	before := strings.TrimRightFunc(stripped[:paren], textfold.IsSpace)
	j := len(before)
	for j > 0 {
		r, n := utf8.DecodeLastRuneInString(before[:j])
		if !pycompat.IsWordRune(r) {
			break
		}
		j -= n
	}
	if j == len(before) {
		return m, 0, false
	}
	name := before[j:]
	prefix := strings.TrimRightFunc(before[:j], textfold.IsSpace)
	first, _ := utf8.DecodeRuneInString(name)
	if notMethodNames[name] || !(pycompat.IsAlpha(first) || first == '_') {
		return m, 0, false
	}
	if strings.HasSuffix(prefix, ".") || strings.Contains(prefix, "=") {
		return m, 0, false
	}
	var mods []string
	returnType := ""
	if prefix != "" {
		tokens := textfold.Fields(prefix)
		typeStart := 0
		for k, tok := range tokens {
			if tok == "new" {
				return m, 0, false
			}
			if typeStart == k { // still in the annotations and modifiers
				if strings.HasPrefix(tok, "@") {
					typeStart = k + 1
				} else if methodModifiers[tok] {
					mods = append(mods, tok)
					typeStart = k + 1
				}
			}
		}
		returnType = strings.Join(tokens[typeStart:], " ")
		if strings.ContainsAny(returnType, "=+/%!&|^~;") {
			return m, 0, false
		}
	}
	after := stripped[paren+1:]
	var params, rest string
	if c := strings.IndexByte(after, ')'); c >= 0 {
		params, rest = pycompat.Strip(after[:c]), pycompat.Strip(after[c+1:])
	} else {
		parts := []string{after}
		for i+extra+1 < len(lines) {
			extra++
			next := pycompat.Strip(lines[i+extra])
			parts = append(parts, next)
			if strings.Contains(next, ")") {
				break
			}
			if extra > 15 {
				return m, 0, false
			}
		}
		full := strings.Join(parts, " ")
		c := strings.IndexByte(full, ')')
		if c < 0 {
			return m, 0, false
		}
		params, rest = pycompat.Strip(full[:c]), pycompat.Strip(full[c+1:])
	}
	if rest != "" && rest[0] != '{' && rest[0] != ';' &&
		!strings.HasPrefix(rest, "throws") && !strings.HasPrefix(rest, "default") {
		return m, 0, false
	}
	var types []string
	for _, p := range splitParams(params) {
		if pt := extractParamType(p); pt != "" {
			types = append(types, pt)
		}
	}
	sig := append([]string{}, mods...)
	if returnType != "" {
		sig = append(sig, returnType)
	}
	sig = append(sig, name+"("+strings.Join(types, ", ")+")")
	return javaMethod{Name: name, Modifiers: mods, ReturnType: returnType, Params: types,
		Signature: strings.Join(sig, " ")}, extra, true
}

// classFrame is one entry of class_stack: a type and the brace depth of its body.
type classFrame struct {
	name  string
	depth int
}

// parseMethods is parse_methods_from_content: every method and constructor
// declaration of content (universal newlines), attributed to the innermost
// type whose body is open, or to fileClass outside any.
func parseMethods(content, fileClass string) []javaMethod {
	lines := strings.Split(content, "\n")
	var methods []javaMethod
	var stack []classFrame
	depth := 0
	inBlock := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if inBlock {
			end := strings.Index(line, "*/")
			if end < 0 {
				continue
			}
			line, inBlock = line[end+2:], false
		}
		line, inBlock = dropBlockComments(line, inBlock)
		stripped := pycompat.Strip(line)
		opens, closes := countBraces(line)
		for len(stack) > 0 && depth-closes <= stack[len(stack)-1].depth-1 {
			stack = stack[:len(stack)-1]
		}
		if stripped != "" && !strings.HasPrefix(stripped, "//") && !core.StartsWithAnnotation(stripped) {
			if name, ok := core.TypeDeclName(stripped); ok {
				stack = append(stack, classFrame{name, depth + opens})
			}
		}
		depth += opens - closes
		if stripped == "" || strings.HasPrefix(stripped, "//") || strings.HasPrefix(stripped, "@") ||
			!strings.Contains(stripped, "(") || !(line[0] == ' ' || line[0] == '\t') {
			continue
		}
		m, extra, ok := tryParseMethod(stripped, lines, i)
		if !ok {
			continue
		}
		m.Class = fileClass
		if len(stack) > 0 {
			m.Class = stack[len(stack)-1].name
		}
		switch {
		case m.Name == m.Class:
			m.IndexedName = "<init>"
		case m.ReturnType == "" && len(m.Modifiers) == 0:
			continue // an enum constant, not a method (its continuation lines are scanned again)
		default:
			m.IndexedName = m.Name
		}
		m.Line = i + 1
		methods = append(methods, m)
		i += extra
	}
	return methods
}

// dropBlockComments removes the /* ... */ comments of line, each closed one
// replaced by a space; an unclosed one cuts the line and opens a block.
func dropBlockComments(line string, inBlock bool) (string, bool) {
	for {
		start := strings.Index(line, "/*")
		if start < 0 {
			return line, inBlock
		}
		end := strings.Index(line[start+2:], "*/")
		if end < 0 {
			return line[:start], true
		}
		line = line[:start] + " " + line[start+2+end+2:]
	}
}
