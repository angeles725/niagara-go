package pyre

import (
	"github.com/dlclark/regexp2"
)

// Regexp is a compiled pattern.
type Regexp struct {
	re      *regexp2.Regexp
	pattern string
	names   map[string]int
	groups  int
}

// Compile is re.compile(pattern, flags). The error is an *Error.
func Compile(pattern string, flags Flags) (*Regexp, error) {
	seq, st, err := validate(pattern, flags)
	if err != nil {
		return nil, err
	}
	var em emitter
	em.seq(seq, st.flags)
	opts := regexp2.RegexOptions(regexp2.None)
	if st.flags&IGNORECASE != 0 {
		opts |= regexp2.IgnoreCase
	}
	if st.flags&MULTILINE != 0 {
		opts |= regexp2.Multiline
	}
	if st.flags&DOTALL != 0 {
		opts |= regexp2.Singleline
	}
	re, cerr := regexp2.Compile(em.b.String(), opts)
	if cerr != nil {
		return nil, newError("unsupported by the Go regex engine: "+cerr.Error(), "", -1)
	}
	names := map[string]int{}
	for name, gid := range st.groupdict {
		names[name] = gid
	}
	return &Regexp{re: re, pattern: pattern, names: names, groups: st.groups() - 1}, nil
}

// MustCompile is Compile for patterns known to be valid; it panics otherwise.
func MustCompile(pattern string, flags Flags) *Regexp {
	re, err := Compile(pattern, flags)
	if err != nil {
		panic("pyre: " + pattern + ": " + err.Error())
	}
	return re
}

// String is the Python pattern.
func (r *Regexp) String() string { return r.pattern }

// Groups is the number of capturing groups (pattern.groups).
func (r *Regexp) Groups() int { return r.groups }

// Search is pattern.search(s): the first match, or nil.
func (r *Regexp) Search(s string) *Match { return r.SearchRunes([]rune(s), 0) }

// MatchString reports whether Search finds a match.
func (r *Regexp) MatchString(s string) bool { return r.Search(s) != nil }

// SearchRunes is pattern.search(text, pos) over text already split into runes.
func (r *Regexp) SearchRunes(text []rune, pos int) *Match {
	m, err := r.re.FindRunesMatchStartingAt(text, pos)
	if err != nil || m == nil {
		return nil
	}
	return &Match{m: m, re: r}
}

// FindIter is pattern.finditer over runes: every non-overlapping match in order.
func (r *Regexp) FindIter(text []rune) []*Match {
	var out []*Match
	m, err := r.re.FindRunesMatch(text)
	for err == nil && m != nil {
		out = append(out, &Match{m: m, re: r})
		m, err = r.re.FindNextMatch(m)
	}
	return out
}

// FindAll is pattern.findall for patterns with at most one group: the whole matches,
// or group 1 of each match ("" where it did not take part).
func (r *Regexp) FindAll(s string) []string {
	g := 0
	if r.groups > 0 {
		g = 1
	}
	var out []string
	for _, m := range r.FindIter([]rune(s)) {
		out = append(out, m.Group(g))
	}
	return out
}

// Match is a match object; spans are code-point indices.
type Match struct {
	m  *regexp2.Match
	re *Regexp
}

// Span is match.span(g): (-1, -1) when group g did not take part.
func (m *Match) Span(g int) (int, int) {
	grp := m.m.GroupByNumber(g)
	if grp == nil || len(grp.Captures) == 0 {
		return -1, -1
	}
	return grp.Index, grp.Index + grp.Length
}

// Start is match.start().
func (m *Match) Start() int { s, _ := m.Span(0); return s }

// End is match.end().
func (m *Match) End() int { _, e := m.Span(0); return e }

// Group is match.group(g), "" when the group did not take part (see GroupOK).
func (m *Match) Group(g int) string { s, _ := m.GroupOK(g); return s }

// GroupOK is match.group(g) and whether the group took part (Python returns None).
func (m *Match) GroupOK(g int) (string, bool) {
	grp := m.m.GroupByNumber(g)
	if grp == nil || len(grp.Captures) == 0 {
		return "", false
	}
	return grp.String(), true
}

// Named is match.group(name).
func (m *Match) Named(name string) (string, bool) {
	g, ok := m.re.names[name]
	if !ok {
		return "", false
	}
	return m.GroupOK(g)
}
