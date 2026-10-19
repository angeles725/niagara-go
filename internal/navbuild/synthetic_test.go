package navbuild

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The builders on the synthetic corpus (testdata/organized) against
// testdata/golden/, which holds the normalised (NormalizeVolatile) output of
// the module-navigator Python builders, frozen when D47 retired the Python
// reference. -update rewrites the goldens from the Go builders; review the
// diff before committing.

var update = flag.Bool("update", false, "rewrite testdata/golden from the Go builders (review the diff)")

func goldenPath(index string) string {
	return filepath.Join("testdata", "golden", index+".golden")
}

// newBase is an empty module-navigator base dir (with indexes/, as the
// builders require of an explicit --base-dir).
func newBase(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// syntheticCorpus is testdata/organized, absolute.
func syntheticCorpus(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "organized"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// firstDiff shows the first line where got differs from want.
func firstDiff(want, got []byte) string {
	wl, gl := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d\n  want: %.300q\n  got:  %.300q", i+1, w, g)
		}
	}
	return "only the line structure differs"
}

// checkGolden compares the index file name of base, normalised, with
// golden (the token index record by record); -update rewrites golden.
func checkGolden(t *testing.T, base, name, golden string) {
	t.Helper()
	if name == TokenFile {
		if *update {
			writeGoldenLines(t, golden, tokenDump(IndexPath(base, name)))
		}
		if n, diff := diffSeqs(goldenLines(t, golden), tokenDump(IndexPath(base, name))); diff != "" {
			t.Errorf("%s differs from its golden at record %d: %s", name, n, diff)
		}
		return
	}
	got, err := os.ReadFile(IndexPath(base, name))
	if err != nil {
		t.Fatal(err)
	}
	got = NormalizeVolatile(got)
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from its golden: %s", name, firstDiff(want, got))
	}
}

// TestGoldenSynthetic builds the whole chain (inventory, class index, then
// laterBuilders) on the synthetic corpus and checks every index against its
// golden.
func TestGoldenSynthetic(t *testing.T) {
	base := newBase(t)
	organized := syntheticCorpus(t)
	if _, err := BuildInventory(base, organized); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildClassIndex(base, organized); err != nil {
		t.Fatal(err)
	}
	indexes := []string{InventoryFile, ClassIndexFile}
	for _, b := range laterBuilders {
		if err := b.build(base, organized); err != nil {
			t.Fatal(err)
		}
		indexes = append(indexes, b.index)
	}
	for _, index := range indexes {
		t.Run(index, func(t *testing.T) { checkGolden(t, base, index, goldenPath(index)) })
	}
}
