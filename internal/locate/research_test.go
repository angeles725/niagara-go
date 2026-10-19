package locate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestResearchCheckout: the corpus files (CATALOG.md and a bloque file, both
// regular files at the top level) or the legacy tools/corpus-nav.py mark a
// niagara-research checkout; a half of the corpus pair, or a marker that is a
// directory, does not.
func TestResearchCheckout(t *testing.T) {
	t.Parallel()
	const (
		catalog = "CATALOG.md"
		bloque  = "niagara-mental-model-bloque1.md"
		legacy  = "tools/corpus-nav.py"
	)
	tests := []struct {
		name  string
		files []string // regular files to create
		dirs  []string // directories to create
		want  bool
	}{
		{"corpus files only", []string{catalog, bloque}, nil, true},
		{"named bloque file", []string{catalog, "niagara-mental-model-bloque-test-infrastructure.md"}, nil, true},
		{"legacy marker only", []string{legacy}, nil, true},
		{"both markers", []string{catalog, bloque, legacy}, nil, true},
		{"CATALOG.md without a bloque file", []string{catalog}, nil, false},
		{"a bloque file without CATALOG.md", []string{bloque}, nil, false},
		{"CATALOG.md is a directory", []string{bloque}, []string{catalog}, false},
		{"the bloque file is a directory", []string{catalog}, []string{bloque}, false},
		{"a bloque file below the top level", []string{catalog, "docs/" + bloque}, nil, false},
		{"tools/corpus-nav.py is a directory", nil, []string{legacy}, false},
		{"empty directory", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, d := range tt.dirs {
				mkdir(t, filepath.Join(dir, filepath.FromSlash(d)))
			}
			for _, f := range tt.files {
				p := filepath.Join(dir, filepath.FromSlash(f))
				mkdir(t, filepath.Dir(p))
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := ResearchCheckout(dir); got != tt.want {
				t.Errorf("ResearchCheckout = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestResearchCheckoutIsAFindMarker: Find walks up to the corpus files from a
// nested start.
func TestResearchCheckoutIsAFindMarker(t *testing.T) {
	t.Parallel()
	dir := checkout(t, "CATALOG.md")
	if err := os.WriteFile(filepath.Join(dir, "niagara-mental-model-bloque7.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	o := Options{Getenv: env(nil), Marker: ResearchCheckout, Starts: []Start{FromCwd},
		Getwd: cwdAt(filepath.Join(dir, "bin", "deep"))}
	if got, ok := Find(o); !ok || got != dir {
		t.Errorf("Find = %q %v, want %q true", got, ok, dir)
	}
}

// TestResearchCheckoutSymlinkedBloque: a bloque file that is a symlink to a
// regular file counts (symlinks are followed); a dangling one does not.
func TestResearchCheckoutSymlinkedBloque(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		target bool // create the link's target
		want   bool
	}{{"symlink to a regular file", true, true}, {"dangling symlink", false, false}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			target := filepath.Join(t.TempDir(), "real.md")
			if tt.target {
				if err := os.WriteFile(target, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "CATALOG.md"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, "niagara-mental-model-bloque1.md")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if got := ResearchCheckout(dir); got != tt.want {
				t.Errorf("ResearchCheckout = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestResearchCheckoutBloqueAfterTheFirstBatch: hasBloque reads names 256 at a
// time; a bloque file that is not in the first batch is still found.
func TestResearchCheckoutBloqueAfterTheFirstBatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := range 600 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("other-%03d.md", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"CATALOG.md", "niagara-mental-model-bloque9.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !ResearchCheckout(dir) {
		t.Error("a bloque file among 602 entries was not found")
	}
}
