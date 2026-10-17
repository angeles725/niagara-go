package argparse

import (
	"math"
	"testing"
)

// Python semantics the help and error text depend on; every expectation was printed by
// Python 3.15 (repr, int, float, textwrap.fill, difflib.get_close_matches).

func TestPyRepr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"a", `'a'`}, {"it's", `"it's"`}, {`say "hi"`, `'say "hi"'`}, {`both ' "`, `'both \' "'`},
		{"tab\tnl\n\x01\x7f\u00a0é\u200b😀", "'tab\\tnl\\n\\x01\\x7f\\xa0é\\u200b😀'"}, {`\`, `'\\'`},
		{1.0, "1.0"}, {0.5, "0.5"}, {1e16, "1e+16"}, {1e15, "1000000000000000.0"},
		{1.5e-05, "1.5e-05"}, {0.0001, "0.0001"}, {123456789.125, "123456789.125"}, {-2.5, "-2.5"},
		{1e300, "1e+300"}, {math.SmallestNonzeroFloat64, "5e-324"}, {math.Inf(-1), "-inf"},
		{nil, "None"}, {true, "True"}, {[]any{"x", 1, nil}, "['x', 1, None]"},
	}
	for _, c := range cases {
		if got := pyRepr(c.in); got != c.want {
			t.Errorf("repr(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestParseIntLikePython(t *testing.T) {
	ok := map[string]int64{" 1_000 ": 1000, "+7": 7, "-0": 0, "٣٤": 34}
	for in, want := range ok {
		if got, err := ParseInt(in); err != nil || got != want {
			t.Errorf("int(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"1__0", "_1", "1_", "", "0x10", "1.0"} {
		if _, err := ParseInt(in); err == nil {
			t.Errorf("int(%q) accepted", in)
		}
	}
}

func TestParseFloatLikePython(t *testing.T) {
	ok := map[string]float64{"1e3": 1000, " -.5 ": -0.5, "inf": math.Inf(1), "-Infinity": math.Inf(-1),
		"1_0.5": 10.5, "1e400": math.Inf(1)}
	for in, want := range ok {
		if got, err := ParseFloat(in); err != nil || got != want {
			t.Errorf("float(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if got, err := ParseFloat("nan"); err != nil || !math.IsNaN(got) {
		t.Errorf("float('nan') = %v, %v", got, err)
	}
	for _, in := range []string{"1_e3", "0x1p3", ".", "e5"} {
		if _, err := ParseFloat(in); err == nil {
			t.Errorf("float(%q) accepted", in)
		}
	}
}

func TestFillLikeTextwrap(t *testing.T) {
	got := fillText("Look, goof-ball -- use the -b option! x-ray self-contained well--known", 12, "  ")
	want := "  Look,\n  goof-ball\n  -- use the\n  -b option!\n  x-ray\n  self-\n  contained\n  well--\n  known"
	if got != want {
		t.Errorf("fill:\n%q\nwant\n%q", got, want)
	}
}

func TestCloseMatchLikeDifflib(t *testing.T) {
	for _, c := range []struct {
		word, want string
		choices    []string
	}{{"fats", "fast", []string{"fast", "slow", "auto"}}, {"abc", "abcy", []string{"abcx", "abcy"}},
		{"zz", "", []string{"abcx"}}} {
		if got, _ := closeMatch(c.word, c.choices); got != c.want {
			t.Errorf("close match %q = %q, want %q", c.word, got, c.want)
		}
	}
}
