// Package blocksearch is corpus-nav's find as a library (decision D29): the
// search scopes of the niagara-research corpus, the BM25 ranking with its file
// cache, and the exact text corpus-nav find prints (result headers, hit lines,
// the honest miss line). It never prints: corpus-nav's Main and the nsearch
// front door both render the same Result.
package blocksearch

import (
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Corpus is corpusnav/config.py: where the corpus lives. Paths are kept the way
// pathlib prints them.
type Corpus struct {
	Root, Catalog, Docs, Retros, IndexDir string
	// CacheTag names another tool's find cache in the same IndexDir
	// (find-<scope><tag>.gob): the cache fingerprint holds the stat of the
	// executable, so two binaries sharing one file would rebuild it in turn.
	// corpus-nav itself uses "".
	CacheTag string
}

// New is the corpus at root with the find cache in indexDir (default
// <root>/tools/.corpus-index).
func New(root, indexDir string) *Corpus {
	root = pypath.Str(root)
	if indexDir == "" {
		indexDir = pypath.Join(pypath.Join(root, "tools"), ".corpus-index")
	}
	return &Corpus{Root: root, Catalog: pypath.Join(root, "CATALOG.md"), Docs: pypath.Join(root, "docs"),
		Retros: pypath.Join(root, "retros"), IndexDir: pypath.Str(indexDir)}
}

// DisplayID is corpus.display_id: B720, B1-3, BTI; sub-corpus labels as they are.
func DisplayID(label string) string {
	if blockfiles.IsMainLabel(label) {
		return "B" + label
	}
	return label
}

// MainLabel is findindex._main_label: the main-block label of a display id, or "".
func MainLabel(display string) string {
	if strings.HasPrefix(display, "B") && blockfiles.IsMainLabel(display[1:]) {
		return display[1:]
	}
	return ""
}

// DirFiles is corpus.iter_dir_files: every *.md below dir (pathlib rglob, symlinked
// directories not followed), in Path order (component by component).
func DirFiles(dir string) []string {
	var out []string
	var walk func(d string)
	walk = func(d string) {
		entries, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range entries {
			path := pypath.Join(d, e.Name())
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
	sort.Slice(out, func(i, j int) bool { return pypath.Less(out[i], out[j]) })
	return out
}

// catalogRow is corpus._CATALOG_ROW_RE.match on one line, stripped groups:
//
//	^\|\s*([^|]+?)\s*\|\s*\[[^\]]+\]\([^)]+\)\s*\|\s*(.*?)\s*\|\s*$
//
// written out by hand because every position in it is forced: the label cell ends at the
// first '|' after the leading one, [^\]]+ and [^)]+ end at the first ']' and ')', and the
// title runs to the last '|', which only whitespace may follow (lines carry no line break,
// so '.' matches every character). Both backtracking pyre and Go's RE2 spent 25-90 ms on
// the ~1,400 CATALOG rows of every find; TestCatalogRowMatchesRegex holds it to the regex.
func catalogRow(line string) (label, title string, ok bool) {
	rest, found := strings.CutPrefix(line, "|")
	if !found {
		return "", "", false
	}
	cell, rest, found := strings.Cut(rest, "|")
	if !found || cell == "" { // ([^|]+?) needs one character
		return "", "", false
	}
	// \s*\[[^\]]+\]\([^)]+\)\s*\|
	rest = strings.TrimLeftFunc(rest, textfold.IsSpace)
	if rest, found = strings.CutPrefix(rest, "["); !found {
		return "", "", false
	}
	i := strings.IndexByte(rest, ']')
	if i < 1 {
		return "", "", false
	}
	if rest, found = strings.CutPrefix(rest[i+1:], "("); !found {
		return "", "", false
	}
	if i = strings.IndexByte(rest, ')'); i < 1 {
		return "", "", false
	}
	rest = strings.TrimLeftFunc(rest[i+1:], textfold.IsSpace)
	if rest, found = strings.CutPrefix(rest, "|"); !found {
		return "", "", false
	}
	// \s*(.*?)\s*\|\s*$
	if title, found = strings.CutSuffix(strings.TrimRightFunc(rest, textfold.IsSpace), "|"); !found {
		return "", "", false
	}
	return pycompat.Strip(cell), pycompat.Strip(title), true
}

// LoadCatalog is corpus.load_catalog: CATALOG.md rows as {label: title}.
func (c *Corpus) LoadCatalog() map[string]string {
	titles := map[string]string{}
	text, ok := ReadText(c.Catalog)
	if !ok {
		return titles
	}
	for _, line := range pycompat.SplitLines(text, false) {
		label, title, ok := catalogRow(line)
		if !ok {
			continue
		}
		if label == "Bloque" || label == "--------" || strings.Trim(label, "-") == "" {
			continue
		}
		titles[label] = title
	}
	return titles
}

// ReadText is corpus.read_text_or_none: strict UTF-8 with universal newlines; false
// when the file is missing, unreadable or not UTF-8.
func ReadText(path string) (string, bool) {
	text, err := pycompat.ReadText(path)
	return text, err == nil
}

// ReadH1 is corpus.read_h1: the first stripped "# " line without its marker.
func ReadH1(path string) string {
	text, _ := ReadText(path)
	for _, line := range strings.SplitAfter(text, "\n") {
		if s := pycompat.Strip(line); strings.HasPrefix(s, "# ") {
			return pycompat.Strip(s[2:])
		}
	}
	return ""
}

// TitleFor is corpus.title_for: the CATALOG title, else the H1, else "(no title)".
func TitleFor(label string, catalog map[string]string, path string) string {
	if t := catalog[label]; t != "" {
		return t
	}
	if h1 := ReadH1(path); h1 != "" {
		return h1
	}
	return "(no title)"
}

// Target is one searchable file: its display id and path.
type Target struct{ Display, Path string }

// CannotListLine is the "cannot search" line of a root that cannot be listed (exit 2,
// cli.ExitCannotSearch), as corpus-nav's entry gate prints it.
func CannotListLine(root string, err error) string {
	return "corpus-nav: cannot search: cannot list " + root + ": " + pycompat.StrerrorOf(err)
}

// Targets is corpus.corpus_targets: (display, path) of a search scope. The blocks and
// all scopes list the root; a root that cannot be listed (locked after a caller's entry
// gate) is that error, never a scope without main blocks.
func (c *Corpus) Targets(scope string) ([]Target, error) {
	var out []Target
	if scope == "blocks" || scope == "all" {
		main, err := blockfiles.MainBlocks(c.Root)
		if err != nil {
			return nil, err
		}
		for _, b := range append(main, blockfiles.SubBlocks(c.Root)...) {
			out = append(out, Target{DisplayID(b.Label), b.Path})
		}
	}
	for _, d := range []struct{ scope, dir string }{{"docs", c.Docs}, {"retros", c.Retros}} {
		if scope == d.scope || scope == "all" {
			for _, p := range DirFiles(d.dir) {
				out = append(out, Target{pypath.RelTo(p, c.Root), p})
			}
		}
	}
	return out, nil
}

func fold(s string) string { return textfold.Fold(s, textfold.Corpus) }
