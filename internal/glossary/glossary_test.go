package glossary

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// fixtures is testdata/fixtures.json (python3 -I internal/glossary/testdata/gen.py).
type fixtures struct {
	Canonical struct {
		Lookup        [][]json.RawMessage `json:"lookup"`
		Translations  [][]json.RawMessage `json:"translations"`
		Expand        [][2]string         `json:"expand"`
		WeightedTerms [][]json.RawMessage `json:"weighted_terms"`
		Note          [][2]string         `json:"note"`
		Keys          []string            `json:"keys"`
	} `json:"canonical"`
	ClassRank struct {
		ByStem [][]json.RawMessage `json:"by_stem"`
	} `json:"classrank"`
	Help struct {
		Keys     []string            `json:"keys"`
		Excluded []string            `json:"excluded"`
		Lookup   [][]json.RawMessage `json:"lookup"`
	} `json:"help"`
	Parse map[string]struct {
		Input   string              `json:"input"`
		Entries [][]json.RawMessage `json:"entries"`
		Warned  bool                `json:"warned"`
	} `json:"parse"`
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

func dec(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

func same(a, b []string) bool { return len(a) == 0 && len(b) == 0 || reflect.DeepEqual(a, b) }

// TestEmbeddedCopyMatchesCanonical is the drift test: the embedded data must
// equal niagara-research tools/lib/glossary_es_en.json byte for byte.
func TestEmbeddedCopyMatchesCanonical(t *testing.T) {
	root := os.Getenv("NIAGARA_RESEARCH")
	if root == "" {
		root = "/home/cristian/niagara-research"
	}
	canon, err := os.ReadFile(filepath.Join(root, "tools", "lib", "glossary_es_en.json"))
	if err != nil {
		t.Skipf("canonical glossary not available: %v", err)
	}
	if !bytes.Equal(canon, embedded) {
		t.Fatal("internal/glossary/glossary_es_en.json drifted from tools/lib/glossary_es_en.json: copy it again")
	}
}

func TestCanonicalKeys(t *testing.T) {
	if got, want := Default().keys, load(t).Canonical.Keys; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys: %d, Python %d", len(got), len(want))
	}
}

func TestCanonicalLookupAndQueries(t *testing.T) {
	f := load(t).Canonical
	g := Default()
	for _, c := range f.Lookup {
		var w string
		var want []string
		dec(t, c[0], &w)
		dec(t, c[1], &want)
		if got := g.Lookup(w); !same(got, want) {
			t.Errorf("Lookup(%q) = %q, Python %q", w, got, want)
		}
	}
	for _, c := range f.Translations {
		var q string
		var want [][]json.RawMessage
		dec(t, c[0], &q)
		dec(t, c[1], &want)
		got := g.Translations(q)
		if len(got) != len(want) {
			t.Errorf("Translations(%q): %d, Python %d", q, len(got), len(want))
			continue
		}
		for i, tr := range got {
			var w string
			var terms []string
			dec(t, want[i][0], &w)
			dec(t, want[i][1], &terms)
			if tr.Word != w || !same(tr.Terms, terms) {
				t.Errorf("Translations(%q)[%d] = %v, Python %s %q", q, i, tr, w, terms)
			}
		}
	}
	for _, c := range f.Expand {
		if got := g.Expand(c[0]); got != c[1] {
			t.Errorf("Expand(%q) = %q, Python %q", c[0], got, c[1])
		}
	}
	for _, c := range f.Note {
		if got := g.Note(c[0]); got != c[1] {
			t.Errorf("Note(%q) = %q, Python %q", c[0], got, c[1])
		}
	}
	for _, c := range f.WeightedTerms {
		var q string
		var want [][2]string
		dec(t, c[0], &q)
		dec(t, c[1], &want)
		got := g.WeightedTerms(q)
		if len(got) != len(want) {
			t.Errorf("WeightedTerms(%q) = %v, Python %v", q, got, want)
			continue
		}
		for i, w := range want {
			x, _ := strconv.ParseFloat(w[1], 64)
			if got[i].Term != w[0] || got[i].Weight != x {
				t.Errorf("WeightedTerms(%q)[%d] = %v, Python %v", q, i, got[i], w)
			}
		}
	}
}

func TestClassRankByStem(t *testing.T) {
	want := map[string][]string{}
	for _, e := range load(t).ClassRank.ByStem {
		var k string
		var v []string
		dec(t, e[0], &k)
		dec(t, e[1], &v)
		want[k] = v
	}
	if got := ClassRankByStem(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ClassRankByStem: %d keys, Python %d", len(got), len(want))
	}
}

func TestHelpView(t *testing.T) {
	f := load(t).Help
	if !reflect.DeepEqual(HelpExcluded, f.Excluded) {
		t.Fatalf("HelpExcluded %q, Python EXCLUDED %q", HelpExcluded, f.Excluded)
	}
	h := Help()
	if got := h.keys; !reflect.DeepEqual(got, f.Keys) {
		t.Fatalf("Help keys: %d, Python %d", len(got), len(f.Keys))
	}
	for _, c := range f.Lookup {
		var w string
		var want []string
		dec(t, c[0], &w)
		dec(t, c[1], &want)
		if got := h.LookupKey(w); !same(got, want) {
			t.Errorf("Help LookupKey(%q) = %q, Python %q", w, got, want)
		}
	}
}

func TestParseRules(t *testing.T) {
	for name, c := range load(t).Parse {
		g, bad, err := Parse([]byte(c.Input))
		var entries [][]json.RawMessage
		if err == nil {
			for _, k := range g.keys {
				terms := g.terms[k]
				kb, _ := json.Marshal(k)
				tb, _ := json.Marshal(terms)
				entries = append(entries, []json.RawMessage{kb, tb})
			}
		}
		if len(entries) != len(c.Entries) {
			t.Errorf("%s: %d entries, Python %d", name, len(entries), len(c.Entries))
			continue
		}
		for i := range entries {
			var gk, wk string
			var gt, wt []string
			dec(t, entries[i][0], &gk)
			dec(t, c.Entries[i][0], &wk)
			dec(t, entries[i][1], &gt)
			dec(t, c.Entries[i][1], &wt)
			if gk != wk || !same(gt, wt) {
				t.Errorf("%s entry %d: %s %q, Python %s %q", name, i, gk, gt, wk, wt)
			}
		}
		if warned := err != nil || len(bad) > 0; warned != c.Warned {
			t.Errorf("%s: problem reported %v, Python warned %v (err %v, bad %q)", name, warned, c.Warned, err, bad)
		}
	}
}
