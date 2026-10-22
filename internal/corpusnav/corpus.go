package corpusnav

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpus is corpusnav/config.py plus the corpus.py helpers: where the corpus lives and
// how its files are read. Paths are kept the way pathlib prints them. The search
// scopes, the catalog and the file readers live in blocksearch (shared with nsearch).
type corpus struct {
	root, catalog, docs, retros, indexDir string
	out, err                              io.Writer
	termCache                             bool // semantic's term counts through their cache (Main sets it)
}

func newCorpus(root, indexDir string, out, errw io.Writer) *corpus {
	b := blocksearch.New(root, indexDir)
	return &corpus{root: b.Root, catalog: b.Catalog, docs: b.Docs, retros: b.Retros, indexDir: b.IndexDir,
		out: out, err: errw}
}

// search is the corpus as blocksearch sees it.
func (c *corpus) search() *blocksearch.Corpus {
	return &blocksearch.Corpus{Root: c.root, Catalog: c.catalog, Docs: c.docs, Retros: c.retros, IndexDir: c.indexDir}
}

func pyPath(p string) string         { return pypath.Str(p) }
func pyJoin(a, b string) string      { return pypath.Join(a, b) }
func relTo(path, root string) string { return pypath.RelTo(path, root) }
func dirFiles(dir string) []string   { return blocksearch.DirFiles(dir) }

func displayID(label string) string { return blocksearch.DisplayID(label) }

// blockFile is a blockfiles.Block as the commands use it.
type blockFile struct {
	label, path, corpus string
}

func fromShared(bs []blockfiles.Block) []blockFile {
	out := make([]blockFile, len(bs))
	for i, b := range bs {
		out[i] = blockFile{b.Label, b.Path, b.Corpus}
	}
	return out
}

func (c *corpus) blocks() []blockFile    { return fromShared(blockfiles.MainBlocks(c.root)) }
func (c *corpus) subBlocks() []blockFile { return fromShared(blockfiles.SubBlocks(c.root)) }

// findBlockPath is corpus.find_block_path: the numeric main block numbered n.
func (c *corpus) findBlockPath(n int) (blockFile, bool) {
	want := fmt.Sprint(n)
	for _, b := range c.blocks() {
		if b.label != "1-3" && b.label != "TI" && b.label == want {
			return b, true
		}
	}
	return blockFile{}, false
}

func (c *corpus) loadCatalog() map[string]string { return c.search().LoadCatalog() }

func readText(path string) (string, bool) { return blocksearch.ReadText(path) }

func titleFor(label string, catalog map[string]string, path string) string {
	return blocksearch.TitleFor(label, catalog, path)
}

// extractSection is corpus.extract_section: the lines of a "## <name>" section.
func extractSection(path string, names ...string) []string {
	text, _ := readText(path)
	var out []string
	capturing := false
	for _, line := range pycompat.SplitLines(text, false) {
		if strings.HasPrefix(line, "## ") {
			if capturing {
				break
			}
			heading := textfold.Lower(pycompat.Strip(line[3:]))
			capturing = false
			for _, n := range names {
				capturing = capturing || heading == textfold.Lower(n)
			}
			continue
		}
		if capturing {
			out = append(out, line)
		}
	}
	return out
}

// reportUnreadable is corpus.report_unreadable.
func (c *corpus) reportUnreadable(paths []string) {
	for _, p := range paths {
		fmt.Fprintln(c.err, blocksearch.UnreadableLine(p))
	}
}

type target struct{ display, path string }

// targets is corpus.corpus_targets: (display, path) of a search scope.
func (c *corpus) targets(scope string) []target {
	var out []target
	for _, t := range c.search().Targets(scope) {
		out = append(out, target{t.Display, t.Path})
	}
	return out
}

func baseName(path string) string { return filepath.Base(path) }
