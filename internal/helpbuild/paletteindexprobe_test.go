package helpbuild

import (
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// TestPaletteIndexDryProbe builds palette-index.json from a real install into
// a temporary data root (the install is only read) and prints the palette
// count (indexed + errors) the docs compare with a grep audit of the same
// install. Set NH_PALETTE_INDEX_PROBE to the install folder (the one holding
// modules/) to run it, with -v to see the counts.
func TestPaletteIndexDryProbe(t *testing.T) {
	install := os.Getenv("NH_PALETTE_INDEX_PROBE")
	if install == "" {
		t.Skip("NH_PALETTE_INDEX_PROBE is not set (an install folder to probe read-only)")
	}
	root := t.TempDir()
	res, err := buildPaletteIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range append(res.Log, res.Warnings...) {
		t.Log(l)
	}
	idx, err := index.LoadObject(index.Path(root, paletteIndexName))
	if err != nil {
		t.Fatal(err)
	}
	pals, _ := idx.Get("palettes")
	errs, _ := idx.Get("errors")
	jars := map[string]bool{}
	for _, p := range pals.([]any) {
		j, _ := p.(*index.Object).String("jar")
		jars[j] = true
	}
	t.Logf("palettes: %d indexed + %d errors = %d (in %d jars)", len(pals.([]any)), len(errs.([]any)),
		len(pals.([]any))+len(errs.([]any)), len(jars))
	if st, err := os.Stat(index.Path(root, paletteIndexName)); err == nil {
		t.Logf("palette-index.json: %d bytes", st.Size())
	}
}
