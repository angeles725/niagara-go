package audits

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFiles writes each rel under top with its own name as the text.
func writeFiles(t *testing.T, top string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(top, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// artifactLexicons takes every *.lexicon under extracted/ (os.walk: a directory named *.lexicon
// walked into, a symlinked directory not entered), sorted by relative name; an unreadable name
// still keeps the JAR out, and only when extracted/ names none does the JAR supply them.
func TestArtifactLexiconsWalksExtracted(t *testing.T) {
	top := t.TempDir()
	ext, out := filepath.Join(top, "ext"), filepath.Join(top, "out")
	writeFiles(t, ext, "b.lexicon", "a/z.lexicon", "a.x/c.lexicon", "d.lexicon/e.lexicon", "note.txt")
	writeFiles(t, out, "f.lexicon")
	if err := os.Symlink(out, filepath.Join(ext, "link")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := os.Symlink(filepath.Join(top, "missing"), filepath.Join(ext, "a", "broken.lexicon")); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(top, "m.jar")
	f, err := os.Create(jar)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("x.lexicon")
	_, _ = w.Write([]byte("x"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	rels := func(files []lexiconFile) (got [][2]string) {
		for _, l := range files {
			got = append(got, [2]string{l.rel, l.text})
		}
		return got
	}
	want := [][2]string{{"a.x/c.lexicon", "a.x/c.lexicon"}, {"a/z.lexicon", "a/z.lexicon"},
		{"b.lexicon", "b.lexicon"}, {"d.lexicon/e.lexicon", "d.lexicon/e.lexicon"}}
	if got := rels(artifactLexicons(ext, jar)); !reflect.DeepEqual(got, want) {
		t.Errorf("extracted lexicons\n got %q\nwant %q", got, want)
	}
	only := filepath.Join(top, "only")
	if err := os.MkdirAll(only, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(top, "missing"), filepath.Join(only, "u.lexicon")); err != nil {
		t.Fatal(err)
	}
	if got := artifactLexicons(only, jar); len(got) != 0 {
		t.Errorf("an unreadable name must keep the JAR out, got %q", rels(got))
	}
	if got := rels(artifactLexicons(filepath.Join(top, "absent"), jar)); !reflect.DeepEqual(got, [][2]string{{"x.lexicon", "x"}}) {
		t.Errorf("JAR fallback %q", got)
	}
}
