package corpusnav

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// tools/lib/blockfiles.py, the one block-discovery rule: main blocks at the repo root,
// sub-corpus blocks anywhere below it (organized/, _backups/, tools/, node_modules/,
// niagara-help/, module-navigator/ and dot-dirs skipped). TODO(shared): gen-catalog and
// check-coverage carry the same rule; it belongs in one internal/blockfiles package (not
// in this change's edit surface).

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

// blockFile is blockfiles.BlockFile.
type blockFile struct {
	label, path, corpus string
}

// mainLabel is blockfiles.main_label: "1-3", "TI", "<N>" or "" (not a main block).
func mainLabel(name string) string {
	switch name {
	case consolidated:
		return "1-3"
	case testInfra:
		return "TI"
	}
	if m := mainRe.Search(name); m != nil {
		return pyInt(m.Group(1))
	}
	return ""
}

// pyInt is str(int(digits)) for a run of Unicode decimal digits (re's \d).
func pyInt(digits string) string {
	var b strings.Builder
	for _, r := range digits {
		start := r // Nd digits come in runs of ten starting at zero
		for unicode.IsDigit(start - 1) {
			start--
		}
		b.WriteRune('0' + (r-start)%10)
	}
	if s := strings.TrimLeft(b.String(), "0"); s != "" {
		return s
	}
	return "0"
}

func isDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// sortKey is blockfiles.label_sort_key; numbers compare by value (canonical decimal).
type sortKey struct {
	group     int
	num, text string
}

func labelSortKey(label string) sortKey {
	switch {
	case label == "1-3":
		return sortKey{0, "0", ""}
	case isDigits(label):
		return sortKey{1, pyInt(label), ""}
	case label == "TI":
		return sortKey{2, "0", ""}
	}
	if m := trailingRe.Search(label); m != nil {
		return sortKey{3, pyInt(m.Group(2)), textfold.Lower(m.Group(1))}
	}
	return sortKey{3, "0", textfold.Lower(label)}
}

// less orders keys like the Python tuples: (group, int, "") for 0-2, (3, text, int) for 3.
func (k sortKey) less(o sortKey) bool {
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

func sortBlocks(bs []blockFile, tie func(a, b blockFile) bool) {
	sort.SliceStable(bs, func(i, j int) bool {
		ki, kj := labelSortKey(bs[i].label), labelSortKey(bs[j].label)
		if ki != kj {
			return ki.less(kj)
		}
		return tie(bs[i], bs[j])
	})
}

// mainBlocks is blockfiles.main_blocks: sorted by label, then file name.
func mainBlocks(root string) []blockFile {
	entries, _ := os.ReadDir(root)
	var out []blockFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "niagara-mental-model") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if label := mainLabel(name); label != "" {
			out = append(out, blockFile{label, pyJoin(root, name), "main"})
		}
	}
	sortBlocks(out, func(a, b blockFile) bool { return filepath.Base(a.path) < filepath.Base(b.path) })
	return out
}

// subLabel is blockfiles._sub_label: (corpus, label) of a sub-corpus block, ok false otherwise.
func subLabel(root, path string) (corpus, label string, ok bool) {
	name := filepath.Base(path)
	if mainLabel(name) != "" || snapshotRe.MatchString(name) {
		return "", "", false
	}
	for _, re := range []*pyre.Regexp{subBlockRe, subBloque} {
		if m := re.Search(name); m != nil {
			stem, _ := m.Named("stem")
			if p, known := subPrefix[stem]; known {
				n, _ := m.Named("n")
				return p[0], p[1] + pyInt(n), true
			}
		}
	}
	if m := subBlockRe.Search(name); m != nil {
		stem, _ := m.Named("stem")
		n, _ := m.Named("n")
		return stem, stem + "-B" + pyInt(n), true
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

// subBlocks is blockfiles.sub_blocks: every sub-corpus block below root (os.walk,
// skipped and dot directories pruned, symlinked directories not entered), sorted by
// label then path.
func subBlocks(root string) []blockFile {
	var out []blockFile
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range entries {
			name, path := e.Name(), pyJoin(dir, e.Name())
			isDir := e.IsDir()
			if e.Type()&os.ModeSymlink != 0 {
				if st, err := os.Stat(path); err == nil && st.IsDir() {
					continue // os.walk lists it as a directory but does not enter it
				}
			}
			if isDir {
				if !skipDirs[name] && !strings.HasPrefix(name, ".") {
					dirs = append(dirs, path)
				}
				continue
			}
			if strings.HasSuffix(name, ".md") {
				if corpus, label, ok := subLabel(root, path); ok {
					out = append(out, blockFile{label, path, corpus})
				}
			}
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			walk(d)
		}
	}
	walk(root)
	sortBlocks(out, func(a, b blockFile) bool { return a.path < b.path })
	return out
}

// allBlocks is blockfiles.all_blocks: the main corpus, then every sub-corpus block.
func allBlocks(root string) []blockFile { return append(mainBlocks(root), subBlocks(root)...) }
