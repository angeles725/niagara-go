package navbuild

import (
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
)

// laterBuilders are the builders that read the class index, in reindex.sh
// order: each runs after the inventory and the class index of its base dir.
var laterBuilders = []struct {
	script, index string
	build         func(base, organized string) error
}{
	{"build_inheritance.py", InheritanceFile, func(base, _ string) error { _, err := BuildInheritance(base); return err }},
	{"build_xref.py", XrefFile, func(base, organized string) error { _, err := BuildXref(base, organized); return err }},
}

// TestParityLaterIndexes builds the whole chain on each side (inventory, class
// index, then laterBuilders) and diffs every later index. The Python builders
// run with a random hash seed (-E): the xref key order no longer follows set
// order (module-navigator fix/deterministic-xref-base-dir).
func TestParityLaterIndexes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		corpus func(*testing.T) string
	}{
		{"synthetic", syntheticCorpus},
		{"real-modules", realSubset},
		{"full-corpus", fullCorpus},
	} {
		t.Run(tc.name, func(t *testing.T) {
			organized := tc.corpus(t)
			pyBase, goBase := newBase(t), newBase(t)
			runPython(t, "build_module_inventory.py", pyBase, organized)
			runPython(t, "build_class_index.py", pyBase, organized)
			if _, err := BuildInventory(goBase, organized); err != nil {
				t.Fatal(err)
			}
			if _, err := BuildClassIndex(goBase, organized); err != nil {
				t.Fatal(err)
			}
			for _, b := range laterBuilders {
				pyTime := runPython(t, b.script, pyBase, organized)
				t0 := time.Now()
				if err := b.build(goBase, organized); err != nil {
					t.Fatal(err)
				}
				t.Logf("%s: python %v, go %v", b.index, pyTime.Round(time.Millisecond), time.Since(t0).Round(time.Millisecond))
				sameIndex(t, b.index, pyBase, goBase)
			}
		})
	}
}

func TestReadClassIndex(t *testing.T) {
	for _, tc := range []struct {
		name, doc string
		entries   int
		fails     bool
	}{
		// build_inheritance/build_xref read ci_data.get("classes", {}): a missing key is empty.
		{"no classes key", `{"_meta": {}}`, 0, false},
		{"null members", `{"classes": {"A": [{"outer_class": null, "extends": null, "implements": null, "path": "a/A.java"}]}}`, 1, false},
		{"not JSON", `{"classes": `, 0, true},
		{"not an object", `[]`, 0, true},
		{"classes not an object", `{"classes": []}`, 0, true},
		{"entries not a list", `{"classes": {"A": {}}}`, 0, true},
		{"entry not an object", `{"classes": {"A": [1]}}`, 0, true},
		{"extends not a string", `{"classes": {"A": [{"extends": ["B"]}]}}`, 0, true},
		{"implements not a list", `{"classes": {"A": [{"implements": "I"}]}}`, 0, true},
		{"implements holds a non-string", `{"classes": {"A": [{"implements": [1]}]}}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := newBase(t)
			mustWrite(t, IndexPath(base, ClassIndexFile), tc.doc)
			got, err := readClassIndex(base)
			if (err != nil) != tc.fails || len(got) != tc.entries {
				t.Errorf("got %d entries, err %v; want %d, fails=%v", len(got), err, tc.entries, tc.fails)
			}
		})
	}
	if _, err := readClassIndex(newBase(t)); err == nil || !strings.Contains(err.Error(), "Run ./reindex.sh") {
		t.Errorf("missing class index: %v", err)
	}
}

func TestTruthy(t *testing.T) {
	for _, tc := range []struct {
		doc  string
		want bool
	}{{`null`, false}, {`""`, false}, {`"x"`, true}, {`0`, false}, {`0.0`, false}, {`-0`, false},
		{`2`, true}, {`NaN`, true}, {`1e999`, true}, {`[]`, false}, {`[0]`, true}, {`{}`, false}, {`{"a":1}`, true}, {`false`, false}} {
		v, err := index.Decode([]byte(tc.doc))
		if err != nil {
			t.Fatal(err)
		}
		if got := truthy(v); got != tc.want {
			t.Errorf("truthy(%s) = %v, want %v", tc.doc, got, tc.want)
		}
	}
}
