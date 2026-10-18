package niagaraannot

import (
	"strings"
	"testing"
)

// TestCommentsInValues: a comment inside a multi-line value is not part of its text, so a //
// comment cannot swallow the terms after it, and override is a boolean expression.
func TestCommentsInValues(t *testing.T) {
	ty, err := Parse(`@NiagaraProperty(name = "out", type = "int",
  flags = Flags.SUMMARY // the wire sheet
        | /* not editable */ Flags.READONLY,
  override = ( true ) /* inherited */)
@NiagaraProperty(name = "b", type = "String", defaultValue = "a // b /* c */", override = false)
class A {}`)
	if err != nil {
		t.Fatal(err)
	}
	s := ty.Slots[0]
	if s.Flags != "Flags.SUMMARY | Flags.READONLY" || !s.Override {
		t.Errorf("flags %q override %v; want comments dropped, override true", s.Flags, s.Override)
	}
	if l, err := FlagLetters(s.Flags); l != "rs" || err != nil {
		t.Errorf("letters %q, %v; want rs", l, err)
	}
	if b := ty.Slots[1]; b.Default != "a // b /* c */" || b.Override {
		t.Errorf("string with comment markers: default %q override %v", b.Default, b.Override)
	}
}

// TestValueErrors: values the parser refuses instead of guessing.
func TestValueErrors(t *testing.T) {
	cases := []struct{ name, src, err string }{
		{"override not boolean", `@NiagaraAction(name = "a", override = OVERRIDE)`, "cannot evaluate override OVERRIDE"},
		{"override expression", `@NiagaraAction(name = "a", override = !false)`, "cannot evaluate override !false"},
		{"facet without value", `@NiagaraAction(name = "a", facets = @Facet(name = "k"))`, "@Facet has no value"},
	}
	for _, c := range cases {
		if _, err := Parse(c.src + "\nclass A {}"); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
		}
	}
}

// TestLiterals: text blocks lose their incidental indentation and trailing spaces as javac strips
// them; octal escapes decode as \0..\377.
func TestLiterals(t *testing.T) {
	cases := []struct{ name, lit, want string }{
		{"plain", `"a\tb"`, "a\tb"},
		{"octal one digit", `"\0x"`, "\x00x"},
		{"octal two digits", `"\12x"`, "\nx"},
		{"octal three digits", `"\101\102"`, "AB"},
		{"octal max", `"\377"`, "ÿ"},
		{"octal stops at 3 digits", `"\1011"`, "A1"},
		{"octal 4-7 takes two digits", `"\477"`, "'7"},
		{"unicode", `"A"`, "A"},
		{"block closing on its own line", "\"\"\"\n    a\n      b\n    \"\"\"", "a\n  b\n"},
		{"block closing after content", "\"\"\"\n    a\n    b\"\"\"", "a\nb"},
		{"block closing sets indentation", "\"\"\"\n    a\n  \"\"\"", "  a\n"},
		{"block blank line and trailing spaces", "\"\"\"\n  a   \n\n  b\"\"\"", "a\n\nb"},
		{"block escapes after stripping", "\"\"\"\n  a\\n\\\n  b\"\"\"", "a\nb"},
		// Expected values are String.stripIndent's (JDK 21), which javac applies: Character.isWhitespace
		// counts \v, form feed and U+3000 as white space and U+00A0 as content.
		{"block vertical tab line is blank", "\"\"\"\n  a\n\v\n  b\n  \"\"\"", "a\n\nb\n"},
		{"block no-break space line is content", "\"\"\"\n  a\n\u00a0\n  b\n  \"\"\"", "  a\n\u00a0\n  b\n"},
		{"block blank line does not set indentation", "\"\"\"\n    a\n  \v  \n    b\n    \"\"\"", "a\n\nb\n"},
		{"block trailing white space stops at no-break space", "\"\"\"\n  a\v \u00a0\n  b\u00a0\n  \"\"\"", "a\v \u00a0\nb\u00a0\n"},
		{"block indentation counts characters", "\"\"\"\n  a\n\u3000\u3000b\n  \"\"\"", "a\nb\n"},
		{"block form feed and vertical tab indent", "\"\"\"\n\v\v a\n     b\n  \"\"\"", " a\n   b\n"},
	}
	for _, c := range cases {
		toks, err := lex(c.lit)
		if err != nil || len(toks) != 1 || toks[0].kind != tString || toks[0].text != c.want {
			t.Errorf("%s: %q -> %+v, %v; want %q", c.name, c.lit, toks, err, c.want)
		}
	}
}
