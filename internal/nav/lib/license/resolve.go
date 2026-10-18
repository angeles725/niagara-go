package license

import (
	"database/sql"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/nav/lib/javatext"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// resolver is _Resolver: call arguments to strings (literals, constants,
// String name() methods, the module name).
type resolver struct {
	root    string
	db      *sql.DB
	files   map[string]*string  // rel -> text (nil: unreadable)
	byClass map[string][]string // class name -> paths (token index files table)
	err     error               // the first database error
}

func newResolver(root string, db *sql.DB) *resolver {
	return &resolver{root: root, db: db, files: map[string]*string{}, byClass: map[string][]string{}}
}

// text is _text: the source of rel, ok=false when it cannot be read.
func (r *resolver) text(rel string) (string, bool) {
	t, seen := r.files[rel]
	if !seen {
		if s, err := pycompat.ReadTextReplace(filepath.Join(r.root, rel)); err == nil {
			t = &s
		}
		r.files[rel] = t
	}
	if t == nil {
		return "", false
	}
	return *t, true
}

func first2(p string) []string {
	parts := strings.SplitN(p, "/", 3)
	return parts[:min(2, len(parts))]
}

func samePrefix(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// classFiles is _class_files: the source files of class name, the caller's
// submodule first, then its module, then the rest, each in path order.
func (r *resolver) classFiles(name, rel string) []string {
	paths, seen := r.byClass[name]
	if !seen {
		rows, err := r.db.Query("SELECT path FROM files WHERE class_name = ?", name)
		if err == nil {
			for rows.Next() {
				var p string
				if err = rows.Scan(&p); err != nil {
					break
				}
				paths = append(paths, p)
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
		}
		if err != nil && r.err == nil {
			r.err = err
		}
		r.byClass[name] = paths
	}
	caller := first2(rel)
	out := append([]string(nil), paths...)
	key := func(p string) (bool, bool) {
		f := first2(p)
		return !samePrefix(f, caller), f[0] != caller[0]
	}
	sort.SliceStable(out, func(a, b int) bool {
		a1, a2 := key(out[a])
		b1, b2 := key(out[b])
		if a1 != b1 {
			return !a1
		}
		if a2 != b2 {
			return !a2
		}
		return out[a] < out[b]
	})
	return out
}

// stringMethod is _string_method: the literal returned by
// `String method() { return "x"; }` in rel or its superclasses (three levels).
func (r *resolver) stringMethod(rel, method string, depth int) *string {
	text, _ := r.text(rel)
	if v, ok := returnedLiteral(text, method); ok {
		return &v
	}
	if parent := superclass(text); parent != "" && depth < 3 {
		for _, p := range r.classFiles(parent[strings.LastIndex(parent, ".")+1:], rel) {
			if v := r.stringMethod(p, method, depth+1); v != nil {
				return v
			}
		}
	}
	return nil
}

// returnedLiteral is re.search(r'\bString\s+' + escape(method) +
// r'\s*\(\s*\)\s*\{\s*return\s+"((?:[^"\\]|\\.)*)"\s*;', text).
func returnedLiteral(text, method string) (string, bool) {
	var out string
	found := false
	occurrences(text, "String", func(p int) bool {
		if javatext.WordBefore(text, p) {
			return false
		}
		i := p + len("String")
		j := javatext.SkipSpace(text, i)
		if j == i {
			return false
		}
		i = lit(text, j, method)
		for _, tok := range []string{"(", ")", "{", "return"} {
			if i < 0 {
				return false
			}
			i = lit(text, javatext.SkipSpace(text, i), tok)
		}
		if i < 0 {
			return false
		}
		j = javatext.SkipSpace(text, i)
		if j == i {
			return false
		}
		content, end, ok := stringBody(text, j)
		if !ok || lit(text, javatext.SkipSpace(text, end), ";") < 0 {
			return false
		}
		out, found = content, true
		return true
	})
	return out, found
}

// superclass is re.search(r'\bclass\s+[\w$]+(?:\s*<[^{]*?>)?\s+extends\s+([\w$.]+)', text).group(1),
// "" when there is none.
func superclass(text string) string {
	out := ""
	occurrences(text, "class", func(p int) bool {
		if javatext.WordBefore(text, p) {
			return false
		}
		i := p + len("class")
		j := javatext.SkipSpace(text, i)
		if j == i {
			return false
		}
		k := skipRun(text, j, isIdent)
		if k == j {
			return false
		}
		// (?:\s*<[^{]*?>)? first, the shortest type parameters first.
		if m := javatext.SkipSpace(text, k); m < len(text) && text[m] == '<' {
			for q := m + 1; q < len(text) && text[q] != '{'; q++ {
				if text[q] == '>' {
					if out = extendsAt(text, q+1); out != "" {
						return true
					}
				}
			}
		}
		out = extendsAt(text, k)
		return out != ""
	})
	return out
}

// extendsAt is \s+extends\s+([\w$.]+) at byte i.
func extendsAt(text string, i int) string {
	j := javatext.SkipSpace(text, i)
	if j == i {
		return ""
	}
	k := lit(text, j, "extends")
	if k < 0 {
		return ""
	}
	l := javatext.SkipSpace(text, k)
	if l == k {
		return ""
	}
	e := skipRun(text, l, func(r rune) bool { return isIdent(r) || r == '.' })
	return text[l:e]
}

// constant is _constant: re.search(r'\b' + escape(name) + r'\s*=\s*"((?:[^"\\]|\\.)*)"\s*;', text).
func constant(text, name string) *string {
	var out *string
	occurrences(text, name, func(p int) bool {
		if !boundary(text, p) {
			return false
		}
		eq := javatext.SkipSpace(text, p+len(name))
		if eq >= len(text) || text[eq] != '=' {
			return false
		}
		content, end, ok := stringBody(text, javatext.SkipSpace(text, eq+1))
		if !ok || lit(text, javatext.SkipSpace(text, end), ";") < 0 {
			return false
		}
		out = &content
		return true
	})
	return out
}

// isName is [A-Za-z_$][\w$]*; isIdentRun is [\w$]+.
func isName(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if !('A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || c == '_' || c == '$') {
		return false
	}
	return skipRun(s, 1, isIdent) == len(s)
}

func isIdentRun(s string) bool { return s != "" && skipRun(s, 0, isIdent) == len(s) }

// resolve is _Resolver.resolve: the string an argument stands for, or nil.
func (r *resolver) resolve(arg, rel string) *string {
	if content, end, ok := stringBody(arg, 0); ok && end == len(arg) {
		return &content
	}
	bare := strings.TrimPrefix(arg, "this.")
	if bare == "TYPE.getModule().getModuleName()" {
		module := rel[:max(0, strings.IndexByte(rel+"/", '/'))]
		return &module
	}
	if name, ok := strings.CutSuffix(bare, ")"); ok {
		if name, ok = strings.CutSuffix(rstrip(name), "("); ok && isIdentRun(name) {
			return r.stringMethod(rel, name, 0)
		}
	}
	owner, name := "", arg
	if k := strings.IndexByte(arg, '.'); k >= 0 && isIdentRun(arg[:k]) && isName(arg[k+1:]) {
		owner, name = arg[:k], arg[k+1:]
	} else if !isName(arg) {
		return nil
	}
	if arg == "null" || arg == "true" || arg == "false" {
		return nil
	}
	if owner == "" || owner == "this" || strings.HasSuffix(rel, "/"+owner+".java") {
		text, _ := r.text(rel)
		return constant(text, name)
	}
	for _, p := range r.classFiles(owner, rel) {
		text, _ := r.text(p)
		if v := constant(text, name); v != nil {
			return v
		}
	}
	return nil
}
