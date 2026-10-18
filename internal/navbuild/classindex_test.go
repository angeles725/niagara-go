package navbuild

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func goldenPath(index string) string {
	return filepath.Join("testdata", "golden", index+".golden")
}

// TestParityClassIndex builds the inventory and then the class index on each
// side (the class index reads its base dir's inventory for the ZKM flags) and
// diffs both indexes; "no-inventory" checks zkm_source null.
func TestParityClassIndex(t *testing.T) {
	for _, tc := range []struct {
		name      string
		corpus    func(*testing.T) string
		inventory bool
	}{
		{"synthetic", syntheticCorpus, true},
		{"synthetic-no-inventory", syntheticCorpus, false},
		{"real-modules", realSubset, true},
		{"full-corpus", fullCorpus, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			organized := tc.corpus(t)
			pyBase, goBase := newBase(t), newBase(t)
			if tc.inventory {
				runPython(t, "build_module_inventory.py", pyBase, organized)
				if _, err := BuildInventory(goBase, organized); err != nil {
					t.Fatal(err)
				}
			}
			pyTime := runPython(t, "build_class_index.py", pyBase, organized)
			t0 := time.Now()
			if _, err := BuildClassIndex(goBase, organized); err != nil {
				t.Fatal(err)
			}
			t.Logf("class-index: python %v, go %v", pyTime.Round(time.Millisecond), time.Since(t0).Round(time.Millisecond))
			sameIndex(t, ClassIndexFile, pyBase, goBase)
		})
	}
}

// TestGoldenSynthetic checks the Go builders against the normalised Python
// outputs of the synthetic corpus checked in under testdata/golden (written by
// `go test -run Parity -update` with the Python builders available), so the
// corner cases stay covered where Python is absent.
func TestGoldenSynthetic(t *testing.T) {
	base := newBase(t)
	organized := syntheticCorpus(t)
	if _, err := BuildInventory(base, organized); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildClassIndex(base, organized); err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{InventoryFile, ClassIndexFile} {
		t.Run(index, func(t *testing.T) {
			want, err := os.ReadFile(goldenPath(index))
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(IndexPath(base, index))
			if err != nil {
				t.Fatal(err)
			}
			if got = NormalizeVolatile(got); !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden: %s", index, firstDiff(want, got))
			}
		})
	}
}

// TestRecallOnGoBuiltIndex is the B2 gate: the recall oracle (`selftest
// --recall`: every declaration in organized/ found by `class`, with its
// supertypes) passes on a Go-built full-corpus class index. The Go mnav has no
// selftest yet, so the Python oracle reads the Go index through --base-dir.
// Only with NAVBUILD_PARITY_FULL=1.
func TestRecallOnGoBuiltIndex(t *testing.T) {
	organized := fullCorpus(t)
	python, nav := pythonNav(t)
	base := newBase(t)
	if _, err := BuildInventory(base, organized); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildClassIndex(base, organized); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-E", "-B", filepath.Join(nav, "tools", "module_nav.py"),
		"--base-dir", base, "selftest", "--recall")
	cmd.Env = append(os.Environ(), "NAV_CORPUS_BASE="+organized)
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil || !strings.HasPrefix(string(out), "recall 100.00% ") {
		t.Fatalf("recall oracle on the Go-built index: %v", err)
	}
}

func TestLoadZKMModules(t *testing.T) {
	for _, tc := range []struct {
		name, inventory string
		want            map[string]bool
		found, fails    bool
	}{
		{"absent", "", map[string]bool{}, false, false},
		{"flags", `{"modules": {"a": {"zkm": true}, "b": {"zkm": 0}, "c": {}, "d": {"zkm": "yes"}}}`,
			map[string]bool{"a": true, "d": true}, true, false},
		{"no modules key", `{"_meta": {}}`, map[string]bool{}, true, false},
		{"invalid JSON", `{"modules": `, nil, true, true},
		{"not an object", `[1]`, nil, true, true},
		{"modules not an object", `{"modules": []}`, nil, true, true},
		{"module entry not an object", `{"modules": {"a": 1}}`, nil, true, true},
		{"BOM", "\ufeff{}", nil, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := newBase(t)
			if tc.inventory != "" {
				mustWrite(t, IndexPath(base, InventoryFile), tc.inventory)
			}
			got, found, err := LoadZKMModules(base)
			if (err != nil) != tc.fails || found != tc.found || (!tc.fails && !reflect.DeepEqual(got, tc.want)) {
				t.Errorf("got %v %v %v, want %v %v fails=%v", got, found, err, tc.want, tc.found, tc.fails)
			}
		})
	}
}
