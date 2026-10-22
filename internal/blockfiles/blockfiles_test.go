package blockfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	for _, b := range listed(t)(AllBlocks(root)) {
		rel, _ := filepath.Rel(root, b.Path)
		got = append(got, [3]string{b.Label, filepath.ToSlash(rel), b.Corpus})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllBlocks =\n%v\nwant\n%v", got, want)
	}
	t.Chdir(root)
	if b := listed(t)(MainBlocks(".")); len(b) == 0 || b[0].Path != "niagara-mental-model.md" {
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
	for _, b := range listed(t)(AllBlocks(root)) {
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

// TestPyInt: str(int(digits)) on the ASCII fast path and on the big.Int path (Unicode
// digits, more than 18 digits, nothing to parse).
func TestPyInt(t *testing.T) {
	for in, want := range map[string]string{"7": "7", "007": "7", "0": "0", "000": "0", "1416": "1416",
		"٣": "3", "0٣": "3", "３２": "32", "": "0", "12345678901234567890": "12345678901234567890",
		"000000000000000000000001": "1"} {
		if got := PyInt(in); got != want {
			t.Errorf("PyInt(%q) = %q, want %q", in, got, want)
		}
	}
}

// lockedRoot is a directory this user cannot list (mode 000, restored after the test).
func lockedRoot(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	d := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(d, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
	if _, err := os.ReadDir(d); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	return d
}

// TestMainBlocksUnlistableRootIsAnError: a root that cannot be listed is not a corpus of
// zero blocks (Path.glob would say so); MainBlocks and AllBlocks fail closed.
func TestMainBlocksUnlistableRootIsAnError(t *testing.T) {
	root := lockedRoot(t)
	if bs, err := MainBlocks(root); err == nil || !errors.Is(err, fs.ErrPermission) || bs != nil {
		t.Errorf("MainBlocks = %v, %v; want a permission error", bs, err)
	}
	if bs, err := AllBlocks(root); err == nil || !errors.Is(err, fs.ErrPermission) || bs != nil {
		t.Errorf("AllBlocks = %v, %v; want a permission error", bs, err)
	}
}

// TestSubBlocksUnlistableRootIsAnError: a root that cannot be listed is not a sub-corpus
// of zero blocks (os.walk would say so); SubBlocks fails closed, with or without the walk
// cache, and caches nothing. An unlistable directory below the root is still skipped.
func TestSubBlocksUnlistableRootIsAnError(t *testing.T) {
	root := lockedRoot(t)
	if bs, err := SubBlocks(root); err == nil || !errors.Is(err, fs.ErrPermission) || bs != nil {
		t.Errorf("SubBlocks = %v, %v; want a permission error", bs, err)
	}
	cached := cacheRoot(t, "a/x-block1.md")
	if err := os.Chmod(cached, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cached, 0o755) })
	for range 2 {
		if bs, err := SubBlocks(cached); err == nil || !errors.Is(err, fs.ErrPermission) || bs != nil {
			t.Errorf("cached SubBlocks = %v, %v; want a permission error", bs, err)
		}
	}
	if _, err := os.Stat(cachePath(cached)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an unlistable root left a walk cache (stat: %v)", err)
	}
	if err := os.Chmod(cached, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(cached, "a"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(cached, "a"), 0o755) })
	if bs, err := SubBlocks(cached); err != nil || len(bs) != 0 {
		t.Errorf("SubBlocks with an unlistable subdirectory = %v, %v; want none, nil", bs, err)
	}
}

// TestMainBlocksMissingRootIsEmpty: a root that does not exist (or is a file) still has
// no blocks and no error, as Path.glob says; the callers' empty-corpus gates handle it.
func TestMainBlocksMissingRootIsEmpty(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{filepath.Join(dir, "gone"), file} {
		if bs, err := MainBlocks(root); err != nil || len(bs) != 0 {
			t.Errorf("MainBlocks(%s) = %v, %v; want none, nil", root, bs, err)
		}
		if bs, err := AllBlocks(root); err != nil || len(bs) != 0 {
			t.Errorf("AllBlocks(%s) = %v, %v; want none, nil", root, bs, err)
		}
		if bs, err := SubBlocks(root); err != nil || len(bs) != 0 {
			t.Errorf("SubBlocks(%s) = %v, %v; want none, nil", root, bs, err)
		}
	}
}

// listed returns the blocks of a listing that cannot fail on a test tree.
func listed(t *testing.T) func([]Block, error) []Block {
	return func(bs []Block, err error) []Block {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return bs
	}
}
