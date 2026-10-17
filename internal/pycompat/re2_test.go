package pycompat

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestRE2SpaceClassIsIsSpace matches the class against every code point (surrogates have no
// UTF-8 form; IsSpace must reject them) and requires exactly the IsSpace set.
func TestRE2SpaceClassIsIsSpace(t *testing.T) {
	var all strings.Builder
	var want []rune
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			if IsSpace(r) {
				t.Fatalf("IsSpace(%U) on a surrogate", r)
			}
			continue
		}
		all.WriteRune(r)
		if IsSpace(r) {
			want = append(want, r)
		}
	}
	var got []rune
	for _, m := range regexp.MustCompile("["+RE2SpaceClass+"]").FindAllString(all.String(), -1) {
		r, _ := utf8.DecodeRuneInString(m)
		got = append(got, r)
	}
	if string(got) != string(want) {
		t.Fatalf("RE2SpaceClass matches %U, IsSpace accepts %U", got, want)
	}
}

func TestRE2SpaceRewrites(t *testing.T) {
	cls := RE2SpaceClass
	for _, tc := range []struct{ in, want string }{
		{`a\sb`, `a[` + cls + `]b`},
		{`\s*=\s*`, `[` + cls + `]*=[` + cls + `]*`},
		{`[\s,]`, `[` + cls + `,]`},
		{`[a\s]`, `[a` + cls + `]`},
		{`[^\s]`, `[^` + cls + `]`},
		{`(?:^|[\s\\])x`, `(?:^|[` + cls + `\\])x`},
		{`[]\s]\s`, `[]` + cls + `][` + cls + `]`},
		{`[^]\s]`, `[^]` + cls + `]`},
		{`[[:alpha:]\s]\s`, `[[:alpha:]` + cls + `][` + cls + `]`},
		{`\\s`, `\\s`},              // escaped backslash, then a literal s
		{`\\\s`, `\\[` + cls + `]`}, // escaped backslash, then \s
		{`\d\w\b\p{L}\x{a0}`, `\d\w\b\p{L}\x{a0}`},
		{`\[\s\]`, `\[[` + cls + `]\]`}, // escaped brackets open no class
		{`a\`, `a\`},
		{``, ``},
	} {
		if got := RE2Space(tc.in); got != tc.want {
			t.Errorf("RE2Space(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRE2SpaceMatchesPythonWhitespace(t *testing.T) {
	re := regexp.MustCompile(RE2Space(`^a\sb[\s,]c$`))
	for _, s := range []string{"a b c", "a b,c", "a　b c", "a\x1cb\x1fc", "a\u0085b c"} {
		if !re.MatchString(s) {
			t.Errorf("%q: Python's \\s matches it", s)
		}
	}
	for _, s := range []string{"a\u200bb c", "ab c", "a b\u180ec"} { // U+200B and U+180E are not \s
		if re.MatchString(s) {
			t.Errorf("%q: Python's \\s does not match it", s)
		}
	}
}

func TestRE2SpaceRejectsNegatedClass(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error(`RE2Space(\S) did not panic`)
		}
	}()
	RE2Space(`a\Sb`)
}
