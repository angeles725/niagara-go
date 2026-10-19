package blockfiles

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestAllBlocks pins all_blocks on a synthetic tree; the expected list was computed with
// tools/lib/blockfiles.py on the same tree: main blocks by label then name, the snapshot
// and copies of main names elsewhere left out, skipped and dot directories pruned.
func TestAllBlocks(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"niagara-mental-model.md", "niagara-mental-model-bloque4.md",
		"niagara-mental-model-bloque004-dup.md", "niagara-mental-model-bloque10-topic.md",
		"niagara-mental-model-bloque-test-infrastructure.md", "niagara-mental-model.2026-04-19.md",
		"niagara-mental-model-bloqueX.md", "niagara-reflow-block2.md", "niagara-reflow-block10.md",
		"docs/niagara-mental-model-bloque98.md", "docs/guide.md", "notes/bloque40-palettes.md",
		"notes/sub/bloque7-x.md", "examinacion-optimizer-4.13/os413-bloque3.md", "other/foo-block5.md",
		"loose-bloque-notes.md", ".hidden/x-bloque1.md", "organized/y-bloque2.md", "tools/z-bloque3.md"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := [][3]string{
		{"1-3", "niagara-mental-model.md", "main"},
		{"4", "niagara-mental-model-bloque004-dup.md", "main"},
		{"4", "niagara-mental-model-bloque4.md", "main"},
		{"10", "niagara-mental-model-bloque10-topic.md", "main"},
		{"TI", "niagara-mental-model-bloque-test-infrastructure.md", "main"},
		{"foo-B5", "other/foo-block5.md", "foo"},
		{"loose-bloque-notes", "loose-bloque-notes.md", "loose"},
		{"niagara-mental-model-bloqueX", "niagara-mental-model-bloqueX.md", "loose"},
		{"notes/bloque40-palettes", "notes/bloque40-palettes.md", "notes"},
		{"notes/sub/bloque7-x", "notes/sub/bloque7-x.md", "notes"},
		{"os413-B3", "examinacion-optimizer-4.13/os413-bloque3.md", "os413"},
		{"R2", "niagara-reflow-block2.md", "reflow"},
		{"R10", "niagara-reflow-block10.md", "reflow"},
	}
	var got [][3]string
	for _, b := range AllBlocks(root) {
		rel, _ := filepath.Rel(root, b.Path)
		got = append(got, [3]string{b.Label, filepath.ToSlash(rel), b.Corpus})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllBlocks =\n%v\nwant\n%v", got, want)
	}
	t.Chdir(root)
	if b := MainBlocks("."); len(b) == 0 || b[0].Path != "niagara-mental-model.md" {
		t.Errorf("Path(\".\") / name drops the dot: %v", b)
	}
}

// TestLabels pins label_sort_key order and main_label (values from the Python module).
func TestLabels(t *testing.T) {
	labels := []string{"R10", "R2", "1-3", "TI", "7", "100", "os413-B3", "notes/x", "notes/bloque9", "Zeta",
		"alpha", "b10", "B9", "٣", "0012"}
	SortLabels(labels)
	want := []string{"1-3", "٣", "7", "0012", "100", "TI", "alpha", "B9", "b10", "notes/bloque9", "notes/x",
		"os413-B3", "R2", "R10", "Zeta"}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("sorted labels = %q, want %q", labels, want)
	}
	for name, want := range map[string]string{"niagara-mental-model-bloque007.md": "7",
		"niagara-mental-model-bloque٣-x.md": "3", "niagara-mental-model-bloque5-a_b.md": "5",
		"niagara-mental-model-bloque5-.md": "", "niagara-mental-model-bloqueX.md": ""} {
		if got := MainLabel(name); got != want {
			t.Errorf("MainLabel(%q) = %q, want %q", name, got, want)
		}
	}
	if !IsMainLabel("720") || !IsMainLabel("TI") || IsMainLabel("R2") || !isSnapshot("niagara-mental-model.2026-04-19.md") {
		t.Error("IsMainLabel / isSnapshot")
	}
}

// TestPathIsPathlib: a block path is str(Path(root) / ... / name), so on Windows every
// separator is "\" (sub_blocks walks with os.walk, then Path(cur) / name).
func TestPathIsPathlib(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"niagara-mental-model-bloque4.md", "notes/sub/bloque7-x.md"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, b := range AllBlocks(root) {
		got = append(got, b.Path)
	}
	sep := string(filepath.Separator)
	want := []string{root + sep + "niagara-mental-model-bloque4.md",
		root + sep + "notes" + sep + "sub" + sep + "bloque7-x.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %q, want %q", got, want)
	}
	// The join spellings themselves are pinned by pypath TestJoin.
}
