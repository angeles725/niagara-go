package helpbuild

import (
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// TestTagIndexDryProbe builds tag-index.json from a real install into a
// temporary data root (the install is only read) and prints the dictionary
// files (indexed + errors) the docs compare with a grep audit of the same
// install. Set NH_TAG_INDEX_PROBE to the install folder (the one holding
// modules/) to run it, with -v to see the counts.
func TestTagIndexDryProbe(t *testing.T) {
	install := os.Getenv("NH_TAG_INDEX_PROBE")
	if install == "" {
		t.Skip("NH_TAG_INDEX_PROBE is not set (an install folder to probe read-only)")
	}
	root := t.TempDir()
	res, err := buildTagIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range append(res.Log, res.Warnings...) {
		t.Log(l)
	}
	idx, err := index.LoadObject(index.Path(root, tagIndexName))
	if err != nil {
		t.Fatal(err)
	}
	files, _ := idx.Get("files")
	errs, _ := idx.Get("errors")
	jars := map[string]bool{}
	for _, f := range files.([]any) {
		j, _ := f.(*index.Object).String("jar")
		jars[j] = true
	}
	t.Logf("dictionary files: %d indexed + %d errors = %d (in %d jars)", len(files.([]any)), len(errs.([]any)),
		len(files.([]any))+len(errs.([]any)), len(jars))
	if st, err := os.Stat(index.Path(root, tagIndexName)); err == nil {
		t.Logf("tag-index.json: %d bytes", st.Size())
	}
}
