package pyre

import (
	"reflect"
	"testing"
)

// covered marks the code points (text index = code point) that re's matches cover.
func covered(re *Regexp, text []rune) []bool {
	out := make([]bool, len(text))
	for _, m := range re.FindIter(text) {
		for i := m.Start(); i < m.End(); i++ {
			out[i] = true
		}
	}
	return out
}

// TestIsWordIsSpace checks IsWord / IsSpace against the compiled \w / \s on every
// code point, surrogates included.
func TestIsWordIsSpace(t *testing.T) {
	all := make([]rune, 0x110000)
	for i := range all {
		all[i] = rune(i)
	}
	word, space := covered(MustCompile(`\w+`, 0), all), covered(MustCompile(`\s+`, 0), all)
	for i, r := range all {
		if IsWord(r) != word[i] {
			t.Errorf("IsWord(%U) = %v, \\w says %v", r, IsWord(r), word[i])
		}
		if IsSpace(r) != space[i] {
			t.Errorf("IsSpace(%U) = %v, \\s says %v", r, IsSpace(r), space[i])
		}
	}
	// spot values from Python's re (str patterns)
	for _, r := range []rune{'a', 'Z', '_', '0', '٣', '²', 'ª', '一'} {
		if !IsWord(r) {
			t.Errorf("IsWord(%U) = false", r)
		}
	}
	for _, r := range []rune{'.', '$', '-', ' ', '​', 0xD800, 0xDFFF, -1, 0x110000} {
		if IsWord(r) {
			t.Errorf("IsWord(%U) = true", r)
		}
	}
	for _, r := range []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680, 0x2000, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000} {
		if !IsSpace(r) {
			t.Errorf("IsSpace(%U) = false", r)
		}
	}
	for _, r := range []rune{'\b', 0x1b, '!', 0x200b, 0x180e, 0xfeff, 0xD800, -1} {
		if IsSpace(r) {
			t.Errorf("IsSpace(%U) = true", r)
		}
	}
}

type anchoredCase struct {
	pattern string
	flags   Flags
	text    string
	pos     int
	want    []string // groups 0..n; nil = no match
}

var anchoredCases = []anchoredCase{
	{`a`, 0, "ba", 0, nil},
	{`a`, 0, "ba", 1, []string{"a"}},
	{`\bfoo`, 0, "xfoo", 1, nil},
	{`\bfoo`, 0, "x foo", 2, []string{"foo"}},
	{`\Bfoo`, 0, "xfoo", 1, []string{"foo"}},
	{`(?<=a)b`, 0, "ab", 1, []string{"b"}},
	{`(?<!a)b`, 0, "ab", 1, nil},
	{`^a`, 0, "aa", 0, []string{"a"}},
	{`^a`, 0, "aa", 1, nil},
	{`\Aa`, 0, "aa", 1, nil},
	{`^a`, MULTILINE, "\na", 1, []string{"a"}},
	{`(?m)^a`, 0, "\na", 1, []string{"a"}},
	{`ABC`, IGNORECASE, "xabc", 1, []string{"abc"}},
	{`(?i)abc`, 0, "xABC", 1, []string{"ABC"}},
	{`a.b`, 0, "xa\nb", 1, nil},
	{`a.b`, DOTALL, "xa\nb", 1, []string{"a\nb"}},
	{`(?s)a.b`, 0, "xa\nb", 1, []string{"a\nb"}},
	{`(\w+)=(\d+)?`, 0, "  key=", 2, []string{"key=", "key", ""}},
	{`(?P<k>\w+)=(?P=k)`, 0, ".ab=ab", 1, []string{"ab=ab", "ab"}},
	{`$`, 0, "ab", 2, []string{""}},
	{``, 0, "ab", 1, []string{""}},
	{`a|ab`, 0, "xab", 1, []string{"a"}},
	{`x*`, 0, "aaxx", 2, []string{"xx"}},
}

func TestMatchRunes(t *testing.T) {
	for _, c := range anchoredCases {
		re := MustCompile(c.pattern, c.flags)
		m := re.MatchRunes([]rune(c.text), c.pos)
		var got []string
		if m != nil {
			for g := 0; g <= re.Groups(); g++ {
				got = append(got, m.Group(g))
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q flags %d .match(%q, %d) = %q, want %q", c.pattern, c.flags, c.text, c.pos, got, c.want)
		}
	}
	re := MustCompile(`(?P<k>\w+)`, 0)
	if m := re.MatchRunes([]rune("..key"), 2); m == nil || m.Start() != 2 || m.End() != 5 {
		t.Fatalf("span of %v", m)
	} else if v, ok := m.Named("k"); !ok || v != "key" {
		t.Errorf("Named(k) = %q, %v", v, ok)
	}
}

// TestMatchRunesIsSearchAtPos checks match(pos) == search(pos) when that search starts
// at pos, and nil otherwise, for every position of a few subjects; also through the
// search fallback MatchRunes uses when the anchored program is unavailable.
func TestMatchRunesIsSearchAtPos(t *testing.T) {
	patterns := []string{`\w+`, `\b\w`, `(\w+)\.set\(\s*([A-Za-z_]\w*)\s*,`, `\s+extends\s+(\w+)`,
		`^.`, `.$`, `(?<=\s)\S+`, `a*?b`, `(a)|b`}
	subjects := []string{"", "a.set(b, c)", "ab ab\nb", " x.set( y ,z.set(w,", "é.set(ü,", "class  Foo\textends Bar"}
	for _, p := range patterns {
		re := MustCompile(p, 0)
		slow := *re
		slow.anchored = nil
		for _, s := range subjects {
			text := []rune(s)
			for pos := 0; pos <= len(text); pos++ {
				want := re.SearchRunes(text, pos)
				if want != nil && want.Start() != pos {
					want = nil
				}
				got, viaSearch := re.MatchRunes(text, pos), slow.MatchRunes(text, pos)
				if (viaSearch == nil) != (want == nil) {
					t.Errorf("%q on %q at %d: fallback disagrees", p, s, pos)
				}
				if (got == nil) != (want == nil) {
					t.Errorf("%q on %q at %d: match %v, search-at-pos %v", p, s, pos, got != nil, want != nil)
					continue
				}
				for g := 0; got != nil && g <= re.Groups(); g++ {
					a, b := got.Span(g)
					c, d := want.Span(g)
					if a != c || b != d {
						t.Errorf("%q on %q at %d: group %d span (%d,%d), want (%d,%d)", p, s, pos, g, a, b, c, d)
					}
				}
			}
		}
	}
}
