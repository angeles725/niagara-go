package corpusnav

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpus is corpusnav/config.py plus the corpus.py helpers: where the corpus lives and
// how its files are read. Paths are kept the way pathlib prints them.
type corpus struct {
	root, catalog, docs, retros, indexDir string
	out, err                              io.Writer
}

func newCorpus(root, indexDir string, out, errw io.Writer) *corpus {
	root = pyPath(root)
	if indexDir == "" {
		indexDir = pyJoin(pyJoin(root, "tools"), ".corpus-index")
	}
	return &corpus{root: root, catalog: pyJoin(root, "CATALOG.md"), docs: pyJoin(root, "docs"),
		retros: pyJoin(root, "retros"), indexDir: pyPath(indexDir), out: out, err: errw}
}

// pyPath is str(pathlib.PurePosixPath(p)): repeated and trailing slashes and "."
// components dropped, ".." kept.
func pyPath(p string) string {
	abs := strings.HasPrefix(p, "/")
	var parts []string
	for _, c := range strings.Split(p, "/") {
		if c != "" && c != "." {
			parts = append(parts, c)
		}
	}
	s := strings.Join(parts, "/")
	switch {
	case abs:
		return "/" + s
	case s == "":
		return "."
	}
	return s
}

// pyJoin is Path(a) / b for a relative b.
func pyJoin(a, b string) string {
	if a == "." {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// relTo is str(path.relative_to(root)) for a path below root.
func relTo(path, root string) string {
	if root == "." {
		return path
	}
	return strings.TrimPrefix(path, strings.TrimSuffix(root, "/")+"/")
}

// displayID is corpus.display_id: B720, B1-3, BTI; sub-corpus labels as they are.
func displayID(label string) string {
	if isMainLabel(label) {
		return "B" + label
	}
	return label
}

func isMainLabel(label string) bool { return isDigits(label) || label == "1-3" || label == "TI" }

func (c *corpus) blocks() []blockFile    { return mainBlocks(c.root) }
func (c *corpus) subBlocks() []blockFile { return subBlocks(c.root) }

// dirFiles is corpus.iter_dir_files: every *.md below dir (pathlib rglob, symlinked
// directories not followed), in Path order (component by component).
func dirFiles(dir string) []string {
	var out []string
	var walk func(d string)
	walk = func(d string) {
		entries, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range entries {
			path := pyJoin(d, e.Name())
			if strings.HasSuffix(e.Name(), ".md") {
				out = append(out, path)
			}
			if e.IsDir() {
				walk(path)
			}
		}
	}
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		walk(dir)
	}
	sort.Slice(out, func(i, j int) bool { return pathLess(out[i], out[j]) })
	return out
}

// pathLess is PurePosixPath ordering: by the list of components.
func pathLess(a, b string) bool {
	pa, pb := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

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

var catalogRowRe = pyre.MustCompile(`^\|\s*([^|]+?)\s*\|\s*\[[^\]]+\]\([^)]+\)\s*\|\s*(.*?)\s*\|\s*$`, 0)

// loadCatalog is corpus.load_catalog: CATALOG.md rows as {label: title}.
func (c *corpus) loadCatalog() map[string]string {
	titles := map[string]string{}
	text, ok := readText(c.catalog)
	if !ok {
		return titles
	}
	for _, line := range pycompat.SplitLines(text, false) {
		m := catalogRowRe.Search(line)
		if m == nil {
			continue
		}
		label, title := pycompat.Strip(m.Group(1)), pycompat.Strip(m.Group(2))
		if label == "Bloque" || label == "--------" || strings.Trim(label, "-") == "" {
			continue
		}
		titles[label] = title
	}
	return titles
}

// readText is corpus.read_text_or_none: strict UTF-8 with universal newlines; false
// when the file is missing, unreadable or not UTF-8.
func readText(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(raw) {
		return "", false
	}
	return strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n"), true
}

// readH1 is corpus.read_h1: the first stripped "# " line without its marker.
func readH1(path string) string {
	text, _ := readText(path)
	for _, line := range strings.SplitAfter(text, "\n") {
		if s := pycompat.Strip(line); strings.HasPrefix(s, "# ") {
			return pycompat.Strip(s[2:])
		}
	}
	return ""
}

// titleFor is corpus.title_for: the CATALOG title, else the H1, else "(no title)".
func titleFor(label string, catalog map[string]string, path string) string {
	if t := catalog[label]; t != "" {
		return t
	}
	if h1 := readH1(path); h1 != "" {
		return h1
	}
	return "(no title)"
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
		fmt.Fprintf(c.err, "corpus-nav: cannot read %s (missing, unreadable or not UTF-8)\n", p)
	}
}

type target struct{ display, path string }

// targets is corpus.corpus_targets: (display, path) of a search scope.
func (c *corpus) targets(scope string) []target {
	var out []target
	if scope == "blocks" || scope == "all" {
		for _, b := range append(c.blocks(), c.subBlocks()...) {
			out = append(out, target{displayID(b.label), b.path})
		}
	}
	for _, d := range []struct{ scope, dir string }{{"docs", c.docs}, {"retros", c.retros}} {
		if scope == d.scope || scope == "all" {
			for _, p := range dirFiles(d.dir) {
				out = append(out, target{relTo(p, c.root), p})
			}
		}
	}
	return out
}

// mainLabelOf is findindex._main_label: the main-block label of a display id, or "".
func mainLabelOf(display string) string {
	if strings.HasPrefix(display, "B") && isMainLabel(display[1:]) {
		return display[1:]
	}
	return ""
}

func baseName(path string) string { return filepath.Base(path) }
