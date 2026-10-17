package checkcoverage

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestAllBlocks pins blockfiles.all_blocks on the fixture corpus (expected list computed
// with tools/lib/blockfiles.py): main blocks by label, then sub-corpus blocks; the dated
// snapshot, docs/ copies of main names, dot-dirs and skipped directories are not blocks.
func TestAllBlocks(t *testing.T) {
	root := filepath.Join("testdata", "corpus")
	want := [][3]string{
		{"1-3", "niagara-mental-model.md", "main"},
		{"10", "niagara-mental-model-bloque10.md", "main"},
		{"11", "niagara-mental-model-bloque11.md", "main"},
		{"12", "niagara-mental-model-bloque12.md", "main"},
		{"13", "niagara-mental-model-bloque13.md", "main"},
		{"14", "niagara-mental-model-bloque14.md", "main"},
		{"15", "niagara-mental-model-bloque15-suffixed.md", "main"},
		{"16", "niagara-mental-model-bloque16.md", "main"},
		{"17", "niagara-mental-model-bloque17.md", "main"},
		{"18", "niagara-mental-model-bloque18.md", "main"},
		{"19", "niagara-mental-model-bloque19.md", "main"},
		{"20", "niagara-mental-model-bloque20.md", "main"},
		{"21", "niagara-mental-model-bloque21.md", "main"},
		{"22", "niagara-mental-model-bloque22.md", "main"},
		{"30", "niagara-mental-model-bloque30-alarm.md", "main"},
		{"31", "niagara-mental-model-bloque31.md", "main"},
		{"32", "niagara-mental-model-bloque32.md", "main"},
		{"TI", "niagara-mental-model-bloque-test-infrastructure.md", "main"},
		{"notes/bloque40-palettes", "notes/bloque40-palettes.md", "notes"},
		{"os413-B3", "examinacion-optimizer-4.13/os413-bloque3.md", "os413"},
		{"R2", "niagara-reflow-block2.md", "reflow"},
	}
	var got [][3]string
	for _, b := range allBlocks(root) {
		rel, _ := filepath.Rel(root, b.path)
		got = append(got, [3]string{b.label, filepath.ToSlash(rel), b.corpus})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allBlocks =\n%v\nwant\n%v", got, want)
	}
}

func TestLabels(t *testing.T) {
	labels := []string{"R10", "R2", "1-3", "TI", "7", "100", "os413-B3", "notes/x", "notes/bloque9", "Zeta",
		"alpha", "b10", "B9", "٣", "0012"}
	sort.SliceStable(labels, func(i, j int) bool { return labelSortKey(labels[i]).less(labelSortKey(labels[j])) })
	want := []string{"1-3", "٣", "7", "0012", "100", "TI", "alpha", "B9", "b10", "notes/bloque9", "notes/x",
		"os413-B3", "R2", "R10", "Zeta"}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("sorted labels = %q, want %q", labels, want)
	}
	for name, want := range map[string]string{"niagara-mental-model-bloque007.md": "7",
		"niagara-mental-model-bloque٣-x.md": "3", "niagara-mental-model-bloque5-a_b.md": "5",
		"niagara-mental-model-bloque5-.md": "", "niagara-mental-model-bloqueX.md": ""} {
		if got := mainLabel(name); got != want {
			t.Errorf("mainLabel(%q) = %q, want %q", name, got, want)
		}
	}
}
