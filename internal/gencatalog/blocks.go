package gencatalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Main-corpus discovery of tools/lib/blockfiles.py (main_label, label_sort_key,
// main_blocks). TODO(shared): corpus-nav and check-coverage carry the same rule;
// it belongs in one internal/blockfiles package (not in this change's edit surface).

const (
	consolidated = "niagara-mental-model.md"
	testInfra    = "niagara-mental-model-bloque-test-infrastructure.md"
)

var (
	mainRe     = pyre.MustCompile(`^niagara-mental-model-bloque(\d+)(?:-[\w-]+)?\.md$`, 0)
	trailingRe = pyre.MustCompile(`^(.*?)(\d+)$`, 0)
)

type block struct {
	label, path string
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
		b.WriteRune('0' + digitValue(r))
	}
	s := strings.TrimLeft(b.String(), "0")
	if s == "" {
		return "0"
	}
	return s
}

// digitValue is the value of a Unicode Nd digit: Nd digits come in runs of ten from 0.
func digitValue(r rune) rune {
	start := r
	for unicode.IsDigit(start - 1) {
		start--
	}
	return (r - start) % 10
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
	group int
	num   string
	text  string
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

func numLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// less orders keys like the Python tuples: (group, int, "") for 0-2, (3, text, int) for 3.
func (k sortKey) less(o sortKey) bool {
	if k.group != o.group {
		return k.group < o.group
	}
	if k.group == 3 && k.text != o.text {
		return k.text < o.text
	}
	return numLess(k.num, o.num)
}

func (k sortKey) equal(o sortKey) bool { return k == o }

// mainBlocks is blockfiles.main_blocks: the main blocks directly under root, sorted by
// label then file name.
func mainBlocks(root string) []block {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []block
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "niagara-mental-model") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if label := mainLabel(name); label != "" {
			out = append(out, block{label, filepath.Join(root, name)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ki, kj := labelSortKey(out[i].label), labelSortKey(out[j].label)
		if !ki.equal(kj) {
			return ki.less(kj)
		}
		return filepath.Base(out[i].path) < filepath.Base(out[j].path)
	})
	return out
}
