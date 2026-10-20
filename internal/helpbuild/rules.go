package helpbuild

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The data rules of the whole-install documentation sweep
// (niagara_help_lib/install_rules.py over install_docs.json): doc_like,
// skip_dirs, extract, exclude (first matching rule wins) and course. The
// embedded install_docs.json is the canonical data file (decision D44): it
// started as a byte-for-byte copy of niagara-help's
// tools/niagara_help_lib/install_docs.json, which the Go port retires, so a
// rule changes here and nowhere else.
//
// As in the Python code, "@name" glob lists are expanded only in "unless";
// "jar" and "entry" take globs or lists of globs.

//go:embed install_docs.json
var embeddedRules []byte

// rulesFile names the embedded data file in error messages.
const rulesFile = "install_docs.json"

// globRe compiles a "/"-path glob (install_rules.glob_re): "*" and "?" stay
// inside one path segment, "**" crosses segments and "**/" also matches no
// directory; the match is anchored and case-insensitive like Python's
// re.IGNORECASE. Go's case folding misses one pair Python's sre has: "i"
// also matches "İ" (its simple lower case) and "ı" (sre's case fix), so an
// i literal becomes that explicit class. Non-ASCII pattern literals fold by
// Go's rules (the shipped patterns are ASCII).
func globRe(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`(?i)\A`)
	for i := 0; i < len(pattern); {
		switch rest := pattern[i:]; {
		case strings.HasPrefix(rest, "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(rest, "**"):
			b.WriteString(".*")
			i += 2
		case rest[0] == '*':
			b.WriteString("[^/]*")
			i++
		case rest[0] == '?':
			b.WriteString("[^/]")
			i++
		default:
			r, n := utf8.DecodeRuneInString(rest)
			if r == 'i' || r == 'I' {
				b.WriteString("[Iiİı]")
			} else {
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
			i += n
		}
	}
	b.WriteString(`\z`)
	return regexp.MustCompile(b.String())
}

// Rule is one extract or exclude rule: id, reason and its compiled globs.
type Rule struct {
	ID, Reason            string
	jars, entries, unless []*regexp.Regexp
}

// Matches is Rule.matches: a jar glob, an entry glob and no unless glob.
func (r *Rule) Matches(jarName, entry string) bool {
	return anyMatch(r.jars, jarName) && anyMatch(r.entries, entry) && !anyMatch(r.unless, entry)
}

func anyMatch(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// SkipDir is one install directory the sweep does not walk.
type SkipDir struct{ Dir, Reason string }

// Rules is the parsed data file (install_rules.Rules).
type Rules struct {
	DocExt, DocNames []string // lower-cased
	docDirs          map[string]bool
	SkipDirs         []SkipDir // dict order: first position, last reason
	skip             map[string]string
	Extract, Exclude []*Rule
	course           map[string][]*regexp.Regexp
}

var courseKinds = []string{"text", "code", "pdf"}

func get(v pyjson.Value, key string) (pyjson.Value, error) {
	x, ok := v.Get(key)
	if !ok {
		return x, fmt.Errorf("missing key %q", key)
	}
	return x, nil
}

func getStr(v pyjson.Value, key string) (string, error) {
	x, err := get(v, key)
	if err == nil && x.Kind != pyjson.Str {
		err = fmt.Errorf("%q is not a string", key)
	}
	return x.S, err
}

// strList is a JSON list of strings; one string counts as a list of one
// when one is true (a rule's "jar" and "entry").
func strList(v pyjson.Value, one bool) ([]string, error) {
	if one && v.Kind == pyjson.Str {
		return []string{v.S}, nil
	}
	if v.Kind != pyjson.Array {
		return nil, fmt.Errorf("want a list of strings, got %s", v.TypeName())
	}
	out := make([]string, len(v.A))
	for i, x := range v.A {
		if x.Kind != pyjson.Str {
			return nil, fmt.Errorf("want a list of strings, got a %s item", x.TypeName())
		}
		out[i] = x.S
	}
	return out, nil
}

func compileAll(globs []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(globs))
	for i, g := range globs {
		out[i] = globRe(g)
	}
	return out
}

func newRule(v pyjson.Value, globs map[string][]string) (*Rule, error) {
	if v.Kind != pyjson.Object {
		return nil, fmt.Errorf("a rule is a %s, not a dict", v.TypeName())
	}
	id, err := getStr(v, "id")
	if err != nil {
		return nil, err
	}
	r := &Rule{ID: id}
	if r.Reason, err = getStr(v, "reason"); err != nil {
		return nil, fmt.Errorf("rule %s: %w", id, err)
	}
	jar, ok := v.Get("jar")
	if !ok {
		jar = pyjson.String("*")
	}
	entry, err := get(v, "entry")
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", id, err)
	}
	unless, ok := v.Get("unless")
	if !ok {
		unless = pyjson.Value{Kind: pyjson.Array}
	}
	jars, err1 := strList(jar, true)
	entries, err2 := strList(entry, true)
	unlessGlobs, err3 := strList(unless, false)
	for _, e := range []error{err1, err2, err3} {
		if e != nil {
			return nil, fmt.Errorf("rule %s: %w", id, e)
		}
	}
	var expanded []string
	for _, p := range unlessGlobs {
		if !strings.HasPrefix(p, "@") {
			expanded = append(expanded, p)
			continue
		}
		list, ok := globs[p[1:]]
		if !ok {
			return nil, fmt.Errorf("rule %s: unknown glob list %s (define it under \"globs\")", id, p)
		}
		expanded = append(expanded, list...)
	}
	r.jars, r.entries, r.unless = compileAll(jars), compileAll(entries), compileAll(expanded)
	return r, nil
}

// Parse reads a data file; source names it in errors.
func Parse(data []byte, source string) (*Rules, error) {
	r, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	ids := map[string]int{}
	for _, rule := range append(append([]*Rule{}, r.Extract...), r.Exclude...) {
		ids[rule.ID]++
	}
	var dup []string
	for id, n := range ids {
		if n > 1 {
			dup = append(dup, id)
		}
	}
	if len(dup) > 0 {
		sort.Strings(dup)
		return nil, fmt.Errorf("duplicate rule ids in %s: %s", source, strings.Join(dup, ", "))
	}
	return r, nil
}

func lowerAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = textfold.Lower(s)
	}
	return out
}

func parse(data []byte) (*Rules, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("not UTF-8")
	}
	doc, err := pyjson.Loads([]rune(string(data)))
	if err != nil {
		return nil, err
	}
	docLike, err := get(doc, "doc_like")
	if err != nil {
		return nil, err
	}
	r := &Rules{docDirs: map[string]bool{}, skip: map[string]string{}, course: map[string][]*regexp.Regexp{}}
	ext, err := get(docLike, "extensions")
	if err == nil {
		r.DocExt, err = strList(ext, false)
	}
	if err != nil {
		return nil, fmt.Errorf("doc_like.extensions: %w", err)
	}
	names, err := get(docLike, "names")
	if err == nil {
		r.DocNames, err = strList(names, false)
	}
	if err != nil {
		return nil, fmt.Errorf("doc_like.names: %w", err)
	}
	r.DocExt, r.DocNames = lowerAll(r.DocExt), lowerAll(r.DocNames)
	if dirs, ok := docLike.Get("dirs"); ok {
		list, err := strList(dirs, false)
		if err != nil {
			return nil, fmt.Errorf("doc_like.dirs: %w", err)
		}
		for _, d := range lowerAll(list) {
			r.docDirs[d] = true
		}
	}
	skip, err := get(doc, "skip_dirs")
	if err != nil {
		return nil, err
	}
	if skip.Kind != pyjson.Array {
		return nil, fmt.Errorf("skip_dirs: want a list, got %s", skip.TypeName())
	}
	for _, d := range skip.A {
		dir, err1 := getStr(d, "dir")
		reason, err2 := getStr(d, "reason")
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("skip_dirs: a dir and a reason per item")
		}
		if _, seen := r.skip[dir]; !seen {
			r.SkipDirs = append(r.SkipDirs, SkipDir{Dir: dir})
		}
		r.skip[dir] = reason
	}
	for i := range r.SkipDirs {
		r.SkipDirs[i].Reason = r.skip[r.SkipDirs[i].Dir]
	}
	globs := map[string][]string{}
	if g, ok := doc.Get("globs"); ok && g.Truthy() { // data.get("globs") or {}
		if g.Kind != pyjson.Object {
			return nil, fmt.Errorf("globs: want a dict, got %s", g.TypeName())
		}
		for _, k := range g.Keys {
			list, err := strList(g.M[k], false)
			if err != nil {
				return nil, fmt.Errorf("globs.%s: %w", k, err)
			}
			globs[k] = list
		}
	}
	if r.Extract, err = rules(doc, "extract", globs); err != nil {
		return nil, err
	}
	if c, ok := doc.Get("course"); ok && c.Truthy() { // data.get("course") or {}
		if c.Kind != pyjson.Object {
			return nil, fmt.Errorf("course: want a dict, got %s", c.TypeName())
		}
		for _, kind := range courseKinds {
			if v, ok := c.Get(kind); ok {
				list, err := strList(v, false)
				if err != nil {
					return nil, fmt.Errorf("course.%s: %w", kind, err)
				}
				r.course[kind] = compileAll(list)
			}
		}
	}
	if r.Exclude, err = rules(doc, "exclude", globs); err != nil {
		return nil, err
	}
	return r, nil
}

func rules(doc pyjson.Value, key string, globs map[string][]string) ([]*Rule, error) {
	list, err := get(doc, key)
	if err != nil {
		return nil, err
	}
	out := make([]*Rule, 0, len(list.A))
	for _, v := range list.A {
		rule, err := newRule(v, globs)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, nil
}

var defaultRules = sync.OnceValues(func() (*Rules, error) { return Parse(embeddedRules, rulesFile) })

// Default is the embedded install_docs.json, parsed once per process.
func Default() (*Rules, error) { return defaultRules() }

// IsDocLike: a documentation-like extension or file name, or any folder
// segment named like a documentation folder (case-insensitive).
func (r *Rules) IsDocLike(entry string) bool {
	low := textfold.Lower(entry)
	for _, e := range r.DocExt {
		if strings.HasSuffix(low, e) {
			return true
		}
	}
	parts := strings.Split(entry, "/")
	last := textfold.Lower(parts[len(parts)-1])
	for _, n := range r.DocNames {
		if last == n {
			return true
		}
	}
	for _, p := range parts[:len(parts)-1] {
		if r.docDirs[textfold.Lower(p)] {
			return true
		}
	}
	return false
}

func firstMatch(rules []*Rule, jarName, entry string) *Rule {
	for _, rule := range rules {
		if rule.Matches(jarName, entry) {
			return rule
		}
	}
	return nil
}

// ExtractRule is the extract rule that makes this module-jar entry a guide
// page, or nil.
func (r *Rules) ExtractRule(jarName, entry string) *Rule {
	return firstMatch(r.Extract, jarName, entry)
}

// ExcludeRule is the first exclude rule matching this entry, or nil.
func (r *Rules) ExcludeRule(jarName, entry string) *Rule {
	return firstMatch(r.Exclude, jarName, entry)
}

// CourseKind is "text", "code", "pdf" or "" (None) for a course-tree path.
func (r *Rules) CourseKind(rel string) string {
	for _, kind := range courseKinds {
		if anyMatch(r.course[kind], rel) {
			return kind
		}
	}
	return ""
}

// SkipReason is the reason install directory dir is not walked.
func (r *Rules) SkipReason(dir string) (string, bool) {
	reason, ok := r.skip[dir]
	return reason, ok
}
