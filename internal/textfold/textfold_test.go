package textfold

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// fixtures is testdata/fixtures.json, written by testdata/gen.py from the
// real Python functions (python3 -I internal/textfold/testdata/gen.py).
type fixtures struct {
	Python    string              `json:"python"`
	Unicode   string              `json:"unicode"`
	FoldOrder []string            `json:"fold_order"`
	Cases     [][]json.RawMessage `json:"cases"`
	Score     [][]json.RawMessage `json:"score"`
	Digests   map[string]string   `json:"digests"`
}

// row is one fixture case: [input, fold per FoldOrder..., lower, parse_query, terms, query_terms].
type row struct {
	in, lower               string
	folds                   []string
	parse, terms, queryTerm []string
}

func rows(t *testing.T, f fixtures) []row {
	t.Helper()
	n := len(f.FoldOrder)
	out := make([]row, len(f.Cases))
	for i, c := range f.Cases {
		if len(c) != n+5 {
			t.Fatalf("case %d has %d fields, want %d", i, len(c), n+5)
		}
		r := &out[i]
		decode(t, c[0], &r.in)
		r.folds = make([]string, n)
		for j := range r.folds {
			decode(t, c[1+j], &r.folds[j])
		}
		decode(t, c[n+1], &r.lower)
		decode(t, c[n+2], &r.parse)
		decode(t, c[n+3], &r.terms)
		decode(t, c[n+4], &r.queryTerm)
	}
	return out
}

func load(t *testing.T) fixtures {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

var presets = map[string]Options{
	"corpus": Corpus, "classrank": ClassRank, "helpfulltext": HelpFulltext, "stripaccents": StripAccents,
}

func TestTableMatchesFixturePython(t *testing.T) {
	f := load(t)
	want := "Python " + f.Python + ", Unicode " + f.Unicode + ";"
	if !strings.Contains(UnicodeHeader(), want) {
		t.Fatalf("unicode.txt header %q, fixtures made with %q: rerun testdata/gen.py", UnicodeHeader(), want)
	}
}

func TestFoldCorpus(t *testing.T) {
	f := load(t)
	if len(f.FoldOrder) != len(presets) {
		t.Fatalf("fixture folds %d, presets %d", len(f.FoldOrder), len(presets))
	}
	for _, r := range rows(t, f) {
		for j, name := range f.FoldOrder {
			o, ok := presets[name]
			if !ok {
				t.Fatalf("fixture fold %q has no preset", name)
			}
			if got := Fold(r.in, o); got != r.folds[j] {
				t.Errorf("%s fold %q = %q, Python %q", name, r.in, got, r.folds[j])
			}
		}
	}
}

// digest hashes each(r) for every non-surrogate code point, like gen.py digest(chars()).
func digest(each func(r rune) string) string {
	h := sha256.New()
	for r := rune(0); r < 0x110000; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		h.Write([]byte(each(r)))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestFoldEveryCodePoint(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive code-point digests")
	}
	f := load(t)
	for name, o := range presets {
		if got := digest(func(r rune) string { return Fold(string(r), o) }); got != f.Digests[name] {
			t.Errorf("%s: digest over every code point differs from Python", name)
		}
	}
	if got := digest(func(r rune) string { return Lower(string(r)) }); got != f.Digests["lower"] {
		t.Error("lower: digest over every code point differs from Python")
	}
	sigma := func(r rune) string {
		c := string(r)
		return Lower("A"+c+"Σ") + Lower(c+"Σ") + Lower("AΣ"+c)
	}
	if got := digest(sigma); got != f.Digests["sigma"] {
		t.Error("final sigma probes: digest differs from Python")
	}
	spaces := sha256.New()
	for r := rune(0); r < 0x110000; r++ {
		if (r < 0xD800 || r > 0xDFFF) && IsSpace(r) {
			spaces.Write([]byte(string(r) + "\n"))
		}
	}
	if hex.EncodeToString(spaces.Sum(nil)) != f.Digests["isspace"] {
		t.Error("isspace: the whitespace set differs from Python")
	}
}

func TestLowerParseQueryAndTerms(t *testing.T) {
	for _, r := range rows(t, load(t)) {
		if got := Lower(r.in); got != r.lower {
			t.Errorf("Lower(%q) = %q, Python %q", r.in, got, r.lower)
		}
		if got := ParseQuery(r.in); !equal(got, r.parse) {
			t.Errorf("ParseQuery(%q) = %q, Python %q", r.in, got, r.parse)
		}
		if got := Terms(r.in); !equal(got, r.terms) {
			t.Errorf("Terms(%q) = %q, Python %q", r.in, got, r.terms)
		}
		if got := QueryTerms(r.in); !equal(got, r.queryTerm) {
			t.Errorf("QueryTerms(%q) = %q, Python %q", r.in, got, r.queryTerm)
		}
	}
}

func TestScore(t *testing.T) {
	for _, c := range load(t).Score {
		var q, text string
		var want int
		decode(t, c[0], &q)
		decode(t, c[1], &text)
		decode(t, c[2], &want)
		if got := Score(ParseQuery(q), text); got != want {
			t.Errorf("Score(%q, %q) = %d, Python %d", q, text, got, want)
		}
	}
}

// equal treats nil and empty as the same list (JSON [] decodes to an empty slice).
func equal(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
