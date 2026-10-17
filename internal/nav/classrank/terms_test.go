package classrank

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestTermsMatchPython replays testdata/terms.json (gen_terms.py): identifier
// terms, text terms, field counts, part counts, squash and query terms of
// class_rank.py on camel-case, accented, compatibility and stop-word inputs.
func TestTermsMatchPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Texts     [][6]json.RawMessage `json:"texts"`
		Questions [][2]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	tm := newTerms()
	for _, row := range fx.Texts {
		var text, sq string
		var ident, text2 []string
		var counts map[string]int
		var n int
		for i, dst := range []any{&text, &ident, &text2, &counts, &n, &sq} {
			if err := json.Unmarshal(row[i], dst); err != nil {
				t.Fatal(err)
			}
		}
		if got := tm.identifier(text); !reflect.DeepEqual(got, ident) {
			t.Errorf("identifier(%q) = %q, Python %q", text, got, ident)
		}
		if got := tm.textTerms(text); !reflect.DeepEqual(got, text2) {
			t.Errorf("textTerms(%q) = %q, Python %q", text, got, text2)
		}
		if got := tm.fieldCounts(text); !reflect.DeepEqual(got, counts) {
			t.Errorf("fieldCounts(%q) = %v, Python %v", text, got, counts)
		}
		if got := partCount(text); got != n {
			t.Errorf("partCount(%q) = %d, Python %d", text, got, n)
		}
		if got := squash(text); got != sq {
			t.Errorf("squash(%q) = %q, Python %q", text, got, sq)
		}
	}
	for _, row := range fx.Questions {
		var q string
		var want []string
		if err := json.Unmarshal(row[0], &q); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(row[1], &want); err != nil {
			t.Fatal(err)
		}
		if got := tm.queryTerms(q); !reflect.DeepEqual(got, want) {
			t.Errorf("queryTerms(%q) = %q, Python %q", q, got, want)
		}
	}
}

func TestIsIdentifier(t *testing.T) {
	for tok, want := range map[string]bool{
		"ackAlarm": true, "BWebServlet": true, "Alarm": false, "alarm": false,
		"_Foo": false, "9Ab": false, "aB": true, "": false,
	} {
		if got := isIdentifier(tok); got != want {
			t.Errorf("isIdentifier(%q) = %v, want %v", tok, got, want)
		}
	}
}
