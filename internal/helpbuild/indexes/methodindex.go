package indexes

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The port of tools/method_indexer.py: indexes/method-index.json, a
// reverse index from member name to every {class, package, section,
// signature} of the METHODS, CONSTRUCTORS and FIELDS sections of
// bajadoc-clean/*.txt. Names are keyed in first-seen order of the sorted
// walk; each list is sorted by (class, signature). The file is
// json.dumps(indent=2, ensure_ascii=False).

// methodMember is one parse_members dict; a nil Class or Package is None.
type methodMember struct {
	Name           string
	Class, Package *string
	Section        string // "methods", "constructors", "fields"
	Signature      string
}

// leadingWord is the length in bytes of the run of \w characters (Python's
// Unicode \w) at the start of s.
func leadingWord(s string) int {
	n := 0
	for n < len(s) {
		r, size := utf8.DecodeRuneInString(s[n:])
		if !pycompat.IsWordRune(r) {
			break
		}
		n += size
	}
	return n
}

// constructorName is re.match(r"(\w+)\s*\(", signature).group(1), "" when
// it does not match.
func constructorName(sig string) string {
	n := leadingWord(sig)
	if n > 0 && strings.HasPrefix(strings.TrimLeftFunc(sig[n:], textfold.IsSpace), "(") {
		return sig[:n]
	}
	return ""
}

// methodName is method_indexer._METHOD_NAME.match(signature).group(1):
// `[^(]*?(\w+)\s*\(`. The lazy prefix cannot hold a "(" and neither can \w
// or \s, so the "(" is the first one; the name is the whole \w run that
// ends right before it (\s allowed in between): starting inside that run
// would also match, but the lazy prefix tries the shortest prefix first.
func methodName(sig string) string {
	p := strings.IndexByte(sig, '(')
	if p < 0 {
		return ""
	}
	head := strings.TrimRightFunc(sig[:p], textfold.IsSpace)
	start := len(head)
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(head[:start])
		if !pycompat.IsWordRune(r) {
			break
		}
		start -= size
	}
	return head[start:]
}

// fieldName is `signature.split(" - ")[0].split()[-1]`, "" for no word.
func fieldName(sig string) string {
	head, _, _ := strings.Cut(sig, " - ")
	parts := strings.FieldsFunc(head, textfold.IsSpace)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

var memberHeads = []string{"CLASS:", "INTERFACE:", "ENUM:", "ANNOTATION TYPE:"}

// parseMembers is method_indexer.parse_members; err is what Python raises
// (the file cannot be read or is not UTF-8): the script does not catch it.
func parseMembers(path string) ([]methodMember, error) {
	var members []methodMember
	var class, pkg *string
	section := ""
	afterColon := func(line string) *string {
		_, v, _ := strings.Cut(line, ":")
		s := pycompat.Strip(v)
		return &s
	}
	err := pyTextLines(path, func(line string) bool {
		line = strings.TrimRight(line, "\n")
		switch {
		case hasAnyPrefix(line, memberHeads):
			class = afterColon(line)
		case strings.HasPrefix(line, "PACKAGE:"):
			pkg = afterColon(line)
		case line == "METHODS:" || line == "CONSTRUCTORS:" || line == "FIELDS:":
			section = strings.TrimRight(line, ":")
		case line == "" || !strings.HasPrefix(line, "  "):
			if section != "" && line != "DESCRIPTION:" {
				section = ""
			}
		case section != "":
			sig := pycompat.Strip(line)
			if sig == "" {
				return true
			}
			var name string
			switch section {
			case "FIELDS":
				name = fieldName(sig)
			case "CONSTRUCTORS":
				name = constructorName(sig)
			default:
				name = methodName(sig)
			}
			if name != "" {
				members = append(members, methodMember{name, class, pkg, strings.ToLower(section), sig})
			}
		}
		return true
	})
	return members, err
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// BuildMethodIndex is method_indexer.build_method_index(baseDir) (the
// script's __main__ with baseDir as its argument). A missing bajadoc-clean/
// gives an empty index, as os.walk yields nothing. err != nil is where
// Python raises (a file that cannot be read or decoded, a failed write:
// traceback, exit 1).
func BuildMethodIndex(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	files := cleanTxtFiles(pycompat.Join(baseDir, "bajadoc-clean"))
	index := map[string][]methodMember{}
	var order []string
	total := 0
	for _, path := range files {
		members, err := parseMembers(path)
		if err != nil {
			return res, err
		}
		for _, m := range members {
			if _, ok := index[m.Name]; !ok {
				order = append(order, m.Name)
			}
			index[m.Name] = append(index[m.Name], m)
			total++
		}
	}
	out := pyjson.NewObject()
	for _, name := range order {
		list := index[name]
		sort.SliceStable(list, func(i, j int) bool {
			if a, b := strOr(list[i].Class), strOr(list[j].Class); a != b {
				return a < b
			}
			return list[i].Signature < list[j].Signature
		})
		vals := make([]pyjson.Value, len(list))
		for i, m := range list {
			vals[i] = pyjson.ObjectOf(pyjson.Field{Key: "class", Value: optValue(m.Class)},
				pyjson.Field{Key: "package", Value: optValue(m.Package)},
				pyjson.Field{Key: "section", Value: pyjson.String(m.Section)},
				pyjson.Field{Key: "signature", Value: pyjson.String(m.Signature)})
		}
		out.Set(name, pyjson.Value{Kind: pyjson.Array, A: vals})
	}
	if err := pyfs.WriteJSON(pycompat.Join(baseDir, "indexes", "method-index.json"), out, pyfs.DumpIndent2Unicode); err != nil {
		return res, err
	}
	res.out("Built method-index.json: %d unique names, %d total members from %d files", len(order), total, len(files))
	return res, nil
}
