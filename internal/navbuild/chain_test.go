package navbuild

import (
	"testing"
	"time"
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
