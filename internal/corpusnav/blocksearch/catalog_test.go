package blocksearch

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// catalogRowRe is corpus._CATALOG_ROW_RE on the Python-faithful engine: the oracle
// catalogRow is held to.
var catalogRowRe = pyre.MustCompile(`^\|\s*([^|]+?)\s*\|\s*\[[^\]]+\]\([^)]+\)\s*\|\s*(.*?)\s*\|\s*$`, 0)

// TestCatalogRowMatchesRegex compares catalogRow with the regex on the fixture CATALOG,
// the real one when $CORPUS_NAV_ROOT names it, edge cases, and random lines over the
// characters the pattern cares about.
func TestCatalogRowMatchesRegex(t *testing.T) {
	lines := []string{
		"| 14 | [a.md](a.md) | Title |", "|14|[a](b)|T|", "| 14 | [a](b) | T | ", "| 14 | [a](b) |  |",
		"| | [a](b) | T |", "|| [a](b) | T |", "| 1 | [](b) | T |", "| 1 | [a]() | T |", "| 1 | [a] (b) | T |",
		"| 1 | [a|x](b|y) | T |", "| 1 | [a](b) | T | U |", "| 1 | [a](b) |", "| 1 | [a](b) | T", "| 1 | x [a](b) | T |",
		"| 1 |　[a](b) | T | ", "| -- | [a](b) | T |", "| Bloque | [a](b) | T |",
		"| 1 | [a](b)) | T |", "| 1 | [[a]](b) | T |", "| 1 | [a](b)c | T |", "", "|", "x| 1 | [a](b) | T |",
	}
	for _, f := range []string{filepath.Join("..", "testdata", "corpus", "CATALOG.md"),
		filepath.Join(os.Getenv("CORPUS_NAV_ROOT"), "CATALOG.md")} {
		if text, ok := ReadText(f); ok {
			lines = append(lines, pycompat.SplitLines(text, false)...)
		}
	}
	// Random rows: the row shape with random cells, then often one character deleted or
	// inserted, so both matches and near misses are frequent.
	rng := rand.New(rand.NewSource(1))
	alphabet := []rune{'|', ' ', ' ', '\t', '\u00a0', '[', ']', '(', ')', 'a', 'b', '-'}
	piece := func(max int) string {
		var b strings.Builder
		for k := rng.Intn(max + 1); k > 0; k-- {
			b.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		return b.String()
	}
	for n := 0; n < 20000; n++ {
		row := []rune("|" + piece(3) + "|" + piece(2) + "[" + piece(3) + "](" + piece(3) + ")" + piece(2) + "|" +
			piece(4) + "|" + piece(2))
		switch i := rng.Intn(len(row)); rng.Intn(3) {
		case 0:
			row = append(row[:i], row[i+1:]...)
		case 1:
			row = append(row[:i], append([]rune{alphabet[rng.Intn(len(alphabet))]}, row[i:]...)...)
		}
		lines = append(lines, string(row))
	}
	matched := 0
	for _, line := range lines {
		label, title, ok := catalogRow(line)
		m := catalogRowRe.Search(line)
		if ok != (m != nil) {
			t.Fatalf("%q: catalogRow ok=%v, regex match=%v", line, ok, m != nil)
		}
		if m == nil {
			continue
		}
		matched++
		if wl, wt := pycompat.Strip(m.Group(1)), pycompat.Strip(m.Group(2)); label != wl || title != wt {
			t.Fatalf("%q: catalogRow (%q, %q), regex (%q, %q)", line, label, title, wl, wt)
		}
	}
	if matched < 500 || len(lines)-matched < 500 {
		t.Errorf("%d of %d lines matched: too few matches or misses to compare", matched, len(lines))
	}
}
