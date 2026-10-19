package fts

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// termsFixture is testdata/terms.json, written by the retired gen_terms.py from
// the real fulltext_terms functions.
type termsFixture struct {
	StopWords []string `json:"stop_words"`
	Cases     []struct {
		Text       string              `json:"text"`
		Fold       string              `json:"fold"`
		Terms      []string            `json:"terms"`
		WordTerms  map[string][]string `json:"word_terms"`
		Stem       map[string]string   `json:"stem"`
		Parse      [][]string          `json:"parse"`
		ParseRelax [][]string          `json:"parse_relax"`
	} `json:"cases"`
}

func loadTermsFixture(t *testing.T) termsFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/terms.json")
	if err != nil {
		t.Fatal(err)
	}
	var f termsFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func groupsOf(gs []Group) [][]string {
	out := [][]string{}
	for _, g := range gs {
		out = append(out, []string(g))
	}
	return out
}

func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

func TestTermsMatchPython(t *testing.T) {
	f := loadTermsFixture(t)
	for _, c := range f.Cases {
		t.Run(c.Text, func(t *testing.T) {
			if got := Fold(c.Text); got != c.Fold {
				t.Errorf("Fold = %q, want %q", got, c.Fold)
			}
			if got := orEmpty(Terms(c.Text)); !reflect.DeepEqual(got, c.Terms) {
				t.Errorf("Terms = %q, want %q", got, c.Terms)
			}
			for w, want := range c.WordTerms {
				if got := WordTerms(w); !reflect.DeepEqual(got, want) {
					t.Errorf("WordTerms(%q) = %q, want %q", w, got, want)
				}
			}
			for w, want := range c.Stem {
				if got := Stem(w); got != want {
					t.Errorf("Stem(%q) = %q, want %q", w, got, want)
				}
			}
			if got := groupsOf(ParseQuery(c.Text, false)); !reflect.DeepEqual(got, c.Parse) {
				t.Errorf("ParseQuery = %q, want %q", got, c.Parse)
			}
			if got := groupsOf(ParseQuery(c.Text, true)); !reflect.DeepEqual(got, c.ParseRelax) {
				t.Errorf("ParseQuery relax = %q, want %q", got, c.ParseRelax)
			}
		})
	}
}

func TestStopWordsMatchPython(t *testing.T) {
	f := loadTermsFixture(t)
	var got []string
	for w := range stopWords {
		got = append(got, w)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, f.StopWords) {
		t.Errorf("stop words differ from search_lang.STOP_WORDS:\n got %q\nwant %q", got, f.StopWords)
	}
}

func TestGroupMatches(t *testing.T) {
	terms := map[string]bool{"lead": true, "lag": true, "registry": true, "alarm": true}
	for _, c := range []struct {
		g    Group
		want bool
	}{
		{Group{"lead lag"}, true},
		{Group{"lead cycl"}, false},
		{Group{"registr*"}, true},
		{Group{"regist"}, false},
		{Group{"zzz", "alarm"}, true},
		{Group{}, false},
	} {
		if got := c.g.Matches(terms); got != c.want {
			t.Errorf("%q.Matches = %v, want %v", c.g, got, c.want)
		}
	}
	if !(Group{"ack"}).MatchesWord("alarmAck") || (Group{"alarma"}).MatchesWord("alarmAck") {
		t.Error("MatchesWord must match whole camelCase parts only")
	}
}

func TestScanWordsOffsetsAreCodePoints(t *testing.T) {
	got := scanWords("é_ab ü1")
	want := []word{{"é", 0}, {"ab", 2}, {"ü1", 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scanWords = %v, want %v", got, want)
	}
}
