package helpbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

// kitControlLexicon is a slice of the kitControl palette lexicon: display
// labels of the blocks, one help sentence.
const kitControlLexicon = "BooleanLatch=Boolean Latch\nBooleanDelay=Boolean Delay\nNumericLatch=Numeric Latch\n" +
	"Latch.description=Latches the input value when the clock input goes true\n"

// A display label is found by find: the label row of kitControl's lexicon
// ranks first for "Boolean Latch", on the page (row) of BooleanLatch,
// above a guide page and help sentences that use the same words.
func TestLexiconLabelRanksForItsDisplayName(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"), [2]string{"kitControl.lexicon", kitControlLexicon})
	testkit.WriteJar(t, filepath.Join(mods, "control-rt.jar"), [2]string{"control.lexicon",
		"BooleanWritable=Boolean Writable\nLatchMode.help=A boolean value can be held by a latch when the mode is set\n"})
	repo := filepath.Join(dir, "repo")
	root := filepath.Join(repo, "versions", "1.0")
	reg, _ := json.Marshal(map[string]any{"default": "1.0", "versions": map[string]any{"1.0": map[string]string{
		"label": "fixture", "install_dir": install, "data_root": "versions/1.0"}}})
	testkit.WriteFile(t, filepath.Join(repo, index.RegistryRel), reg)
	testkit.WriteFile(t, filepath.Join(root, "guides-clean", "kitControl", "latches.txt"),
		[]byte("TITLE: Latch blocks\n\nThe latch blocks hold a boolean or numeric value. A boolean latch output "+
			"follows the input while the clock is true; the numeric latch does the same for numbers.\n"))
	if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := buildLexiconText(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	res, err := fts.Search(root, "", "Boolean Latch", nil, "and")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) == 0 {
		t.Fatal("no hit for Boolean Latch")
	}
	top := res.Hits[0]
	if top.Key() != "lexicon-text/kitControl-rt/kitControl.lexicon.txt" {
		var keys []string
		for _, h := range res.Hits {
			keys = append(keys, h.Key())
		}
		t.Fatalf("top hit %s, want the kitControl lexicon (hits %q)", top.Key(), keys)
	}
	page, err := os.ReadFile(filepath.Join(root, lexiconTextDir, "kitControl-rt", "kitControl.lexicon.txt"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(string(page), "\f") // one full-text page per row; the first holds the header
	if top.Page < 1 || top.Page > len(rows) || !strings.HasSuffix(rows[top.Page-1], "[label] BooleanLatch = Boolean Latch\n") {
		t.Errorf("top hit page %d is not the BooleanLatch label row:\n%s", top.Page, page)
	}
}
