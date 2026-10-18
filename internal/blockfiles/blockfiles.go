// Package blockfiles is tools/lib/blockfiles.py, the one block-discovery rule of the
// niagara-research corpus tools (gen-catalog, check-coverage, corpus-nav): a block on
// disk is seen by every tool or by none.
//
// Main corpus, the repository root only: niagara-mental-model.md ("1-3"),
// niagara-mental-model-bloque<N>[-<suffix>].md ("<N>", leading zeros dropped) and
// niagara-mental-model-bloque-test-infrastructure.md ("TI"); dated snapshots
// niagara-mental-model.<YYYY-MM-DD>.md are not blocks. Sub-corpora, anywhere below:
// <stem>-block<N>.md ("R<N>" for reflow, else "<stem>-B<N>"), os413-bloque<N>.md
// ("os413-B<N>") and any other *bloque*.md (its repo-relative path without ".md").
// Skipped everywhere: organized/, _backups/, tools/, node_modules/, niagara-help/,
// module-navigator/ and dot-dirs. Block paths are joined like pathlib (Path(root) / name).
package blockfiles

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Block is blockfiles.BlockFile.
type Block struct {
	Label  string // "720", "1-3", "TI" (main) or a sub-corpus label ("R2", "os413-B3", "notes/...")
	Path   string
	Corpus string // "main" or the sub-corpus name ("reflow", "os413", "notes", "loose", ...)
}

var skipDirs = map[string]bool{"organized": true, "_backups": true, "tools": true, "node_modules": true,
	"niagara-help": true, "module-navigator": true}

const (
	consolidated = "niagara-mental-model.md"
	testInfra    = "niagara-mental-model-bloque-test-infrastructure.md"
)

var (
	mainRe     = pyre.MustCompile(`^niagara-mental-model-bloque(\d+)(?:-[\w-]+)?\.md$`, 0)
	snapshotRe = pyre.MustCompile(`^niagara-mental-model\.\d{4}-\d{2}-\d{2}\.md$`, 0)
	subBlockRe = pyre.MustCompile(`^(?P<stem>.+)-block(?P<n>\d+)\.md$`, 0)
	subBloque  = pyre.MustCompile(`^(?P<stem>.+)-bloque(?P<n>\d+)\.md$`, 0)
	trailingRe = pyre.MustCompile(`^(.*?)(\d+)$`, 0)
)

// subPrefix maps a sub-corpus file prefix to (corpus, label prefix).
var subPrefix = map[string][2]string{"niagara-reflow": {"reflow", "R"}, "os413": {"os413", "os413-B"}}

// MainLabel is main_label: "1-3", "TI", "<N>" or "" when name is not a main block.
func MainLabel(name string) string {
	switch name {
	case consolidated:
		return "1-3"
	case testInfra:
		return "TI"
	}
	if m := mainRe.Search(name); m != nil {
		return PyInt(m.Group(1))
	}
	return ""
}

// IsMainLabel is is_main_label: a number, "1-3" or "TI".
func IsMainLabel(label string) bool { return IsDigit(label) || label == "1-3" || label == "TI" }

// isSnapshot reports a dated niagara-mental-model.<YYYY-MM-DD>.md snapshot.
func isSnapshot(name string) bool { return snapshotRe.MatchString(name) }

// PyInt is str(int(digits)) for the Unicode decimal digits re's \d matched ("0" if none parse).
func PyInt(digits string) string {
	n, err := pycompat.ParseBigInt(digits)
	if err != nil {
		return "0"
	}
	return n.String()
}

// IsDigit is str.isdigit.
func IsDigit(s string) bool {
	for _, r := range s {
		if !pycompat.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// key is label_sort_key: (0,0,"") for "1-3", (1, int, "") for numbers, (2,0,"") for
// "TI", (3, prefix.lower(), trailing int) otherwise; numbers compare by value.
type key struct {
	group     int
	num, text string
}

func sortKey(label string) key {
	switch {
	case label == "1-3":
		return key{0, "0", ""}
	case IsDigit(label):
		return key{1, PyInt(label), ""}
	case label == "TI":
		return key{2, "0", ""}
	}
	if m := trailingRe.Search(label); m != nil {
		return key{3, PyInt(m.Group(2)), textfold.Lower(m.Group(1))}
	}
	return key{3, "0", textfold.Lower(label)}
}

func (k key) less(o key) bool {
	if k.group != o.group {
		return k.group < o.group
	}
	if k.group == 3 && k.text != o.text {
		return k.text < o.text
	}
	if len(k.num) != len(o.num) {
		return len(k.num) < len(o.num)
	}
	return k.num < o.num
}

// less orders labels by label_sort_key: consolidated first, numbers by value, TI after
// them, sub-corpus labels last.
func less(a, b string) bool { return sortKey(a).less(sortKey(b)) }

// SortLabels sorts labels in place, stably, by label_sort_key.
func SortLabels(labels []string) {
	sort.SliceStable(labels, func(i, j int) bool { return less(labels[i], labels[j]) })
}

func sortBlocks(bs []Block, tie func(a, b Block) bool) {
	sort.SliceStable(bs, func(i, j int) bool {
		ki, kj := sortKey(bs[i].Label), sortKey(bs[j].Label)
		if ki != kj {
			return ki.less(kj)
		}
		return tie(bs[i], bs[j])
	})
}

// join is str(Path(dir) / name) for the host: on Windows pathlib spells every separator
// "\" (the tie-break sort on str(path) then compares that spelling, as Python does).
func join(dir, name string) string { return joinFor(dir, name, runtime.GOOS == "windows") }

func joinFor(dir, name string, windows bool) string {
	sep := "/"
	if windows {
		sep, dir = `\`, strings.ReplaceAll(dir, "/", `\`)
	}
	switch {
	case dir == ".":
		return name
	case strings.HasSuffix(dir, sep):
		return dir + name
	}
	return dir + sep + name
}

// MainBlocks is main_blocks: the main blocks directly under root, sorted by label, then
// file name.
func MainBlocks(root string) []Block {
	entries, _ := os.ReadDir(root)
	var out []Block
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "niagara-mental-model") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if label := MainLabel(name); label != "" {
			out = append(out, Block{label, join(root, name), "main"})
		}
	}
	sortBlocks(out, func(a, b Block) bool { return filepath.Base(a.Path) < filepath.Base(b.Path) })
	return out
}

// subLabel is _sub_label: (corpus, label) of a sub-corpus block, ok false otherwise.
func subLabel(root, path string) (corpus, label string, ok bool) {
	name := filepath.Base(path)
	if MainLabel(name) != "" || isSnapshot(name) {
		return "", "", false // main-corpus names count only at the root (elsewhere they are copies)
	}
	for _, re := range []*pyre.Regexp{subBlockRe, subBloque} {
		if m := re.Search(name); m != nil {
			stem, _ := m.Named("stem")
			if p, known := subPrefix[stem]; known {
				n, _ := m.Named("n")
				return p[0], p[1] + PyInt(n), true
			}
		}
	}
	if m := subBlockRe.Search(name); m != nil {
		stem, _ := m.Named("stem")
		n, _ := m.Named("n")
		return stem, stem + "-B" + PyInt(n), true
	}
	if strings.Contains(name, "bloque") {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", "", false
		}
		rel = filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))
		if first, _, nested := strings.Cut(rel, "/"); nested {
			return first, rel, true
		}
		return "loose", rel, true
	}
	return "", "", false
}

// SubBlocks is sub_blocks: every sub-corpus block below root (os.walk: skipped and dot
// directories pruned, directories sorted, symlinked directories not entered), sorted by
// label, then path.
func SubBlocks(root string) []Block {
	var out []Block
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range entries {
			name, path := e.Name(), join(dir, e.Name())
			if e.Type()&os.ModeSymlink != 0 {
				if st, err := os.Stat(path); err == nil && st.IsDir() {
					continue // os.walk lists it as a directory but does not enter it
				}
			}
			if e.IsDir() {
				if !skipDirs[name] && !strings.HasPrefix(name, ".") {
					dirs = append(dirs, path)
				}
				continue
			}
			if strings.HasSuffix(name, ".md") {
				if corpus, label, ok := subLabel(root, path); ok {
					out = append(out, Block{label, path, corpus})
				}
			}
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			walk(d)
		}
	}
	walk(root)
	sortBlocks(out, func(a, b Block) bool { return a.Path < b.Path })
	return out
}

// AllBlocks is all_blocks: the main corpus, then every sub-corpus block.
func AllBlocks(root string) []Block { return append(MainBlocks(root), SubBlocks(root)...) }
