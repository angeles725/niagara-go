package blockfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// writeTree creates every repo-relative file under root with a one-line body.
func writeTree(tb testing.TB, root string, files []string) {
	tb.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
}

// TestSubBlocksBranches pins sub_blocks on every _sub_label branch: known prefixes for
// -block and -bloque, any other -block stem, loose and nested *bloque* names, main and
// snapshot names below the root, case-sensitive matching, names with no -block/bloque,
// symlinked files (entered) and directories (listed, not entered), dangling links and an
// unreadable directory (skipped silently).
func TestSubBlocksBranches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX tree: symlinks need privilege and a backslash is a separator on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-0 directory, so the unreadable-directory case cannot be built")
	}
	root := t.TempDir()
	writeTree(t, root, []string{
		"niagara-reflow-block3.md", "niagara-reflow-bloque4.md", "os413-block5.md", "os413-bloque06.md",
		"a/b/c-block7.md", "a/b-block1-block2.md", "a/x-block٣.md", "a/x-blockX.md", "a/x-block1.MD",
		"a/y-block5-bloque.md", "a/plain.md", "a/x-Block1.md", "a/x-BLOQUE1.md", "a/blocking.md",
		"a/x-block1.md.txt", "a/x-bloque.txt", "bloque.md", "w/back\\slash-bloque.md",
		"w/niagara-mental-model-bloque9.md", "w/niagara-mental-model.md", "w/niagara-mental-model.2026-01-02.md",
		"w/niagara-mental-model-bloqueZ.md", "target/t-block8.md", "_backups/s-block1.md",
		"module-navigator/m-bloque1.md", ".git/g-block1.md", "locked/l-block1.md", "dir-bloque.md/inner-block2.md",
	})
	for link, target := range map[string]string{"linkdir": "target", "a/link-block9.md": "../target/t-block8.md",
		"a/dangling-block4.md": "missing.md"} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	want := [][3]string{ // label_sort_key order, then path
		{"a/y-block5-bloque", "a/y-block5-bloque.md", "a"},
		{"b-block1-B2", "a/b-block1-block2.md", "b-block1"},
		{"bloque", "bloque.md", "loose"},
		{"c-B7", "a/b/c-block7.md", "c"}, // a -block label is its stem, whatever the nesting
		{"dangling-B4", "a/dangling-block4.md", "dangling"},
		{"inner-B2", "dir-bloque.md/inner-block2.md", "inner"},
		{"link-B9", "a/link-block9.md", "link"},
		{"os413-B5", "os413-block5.md", "os413"},
		{"os413-B6", "os413-bloque06.md", "os413"},
		{"R3", "niagara-reflow-block3.md", "reflow"},
		{"R4", "niagara-reflow-bloque4.md", "reflow"},
		{"t-B8", "target/t-block8.md", "t"},
		{`w/back\slash-bloque`, `w/back\slash-bloque.md`, "w"},
		{"w/niagara-mental-model-bloqueZ", "w/niagara-mental-model-bloqueZ.md", "w"},
		{"x-B3", "a/x-block٣.md", "x"},
	}
	// One worker walks inline only; more read directories concurrently in any order (run
	// with -race to cover the mutex and WaitGroup paths).
	prev := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(prev) })
	for _, procs := range []int{1, 2, 16} {
		runtime.GOMAXPROCS(procs)
		var got [][3]string
		for _, b := range SubBlocks(root) {
			rel, err := filepath.Rel(root, b.Path)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, [3]string{b.Label, filepath.ToSlash(rel), b.Corpus})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GOMAXPROCS %d: SubBlocks =\n%q\nwant\n%q", procs, got, want)
		}
	}
}

// BenchmarkSubBlocks walks a synthetic tree shaped like the corpus (many directories,
// mostly non-block .md and other files, few blocks); BLOCKFILES_BENCH_ROOT adds a run
// on a real corpus root.
func BenchmarkSubBlocks(b *testing.B) {
	b.Run("synthetic", func(b *testing.B) {
		root := b.TempDir()
		var files []string
		for d := 0; d < 400; d++ {
			dir := fmt.Sprintf("library/d%03d/s%d", d, d%7)
			for f := 0; f < 6; f++ {
				files = append(files, fmt.Sprintf("%s/note-%d.md", dir, f), fmt.Sprintf("%s/data-%d.json", dir, f))
			}
			if d%40 == 0 {
				files = append(files, dir+"/topic-block1.md", dir+"/bloque2-x.md")
			}
		}
		writeTree(b, root, files)
		benchWalk(b, root)
	})
	if root := os.Getenv("BLOCKFILES_BENCH_ROOT"); root != "" {
		b.Run("real", func(b *testing.B) { benchWalk(b, root) })
	}
}

func benchWalk(b *testing.B, root string) {
	b.ReportAllocs()
	for b.Loop() {
		if len(SubBlocks(root)) == 0 {
			b.Fatal("no sub-blocks")
		}
	}
}
