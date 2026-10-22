package pycompat

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The expectations are what os.listdir and sorted(os.listdir()) return for the same tree:
// every entry (files, directories, dot names) and an OSError for a missing directory or a file.
func TestListdir(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.txt", "A", ".hidden", "a"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "sub")
	want := []string{".hidden", "A", "a", "b.txt", "sub"}

	got, err := Listdir(dir)
	if err != nil {
		t.Fatalf("Listdir: %v", err)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("Listdir = %q, want the entries %q in any order", got, want)
	}
	if got, err := SortedListdir(dir); err != nil || !slices.Equal(got, want) {
		t.Errorf("SortedListdir = %q, %v, want %q", got, err, want)
	}
	for name, list := range map[string]func(string) ([]string, error){"Listdir": Listdir, "SortedListdir": SortedListdir} {
		if got, err := list(empty); err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s(empty) = %#v, %v, want an empty non-nil list", name, got, err)
		}
		missing := filepath.Join(dir, "missing")
		if got, err := list(missing); got != nil || !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s(missing) = %q, %v, want nil and a not-exist error", name, got, err)
		}
		var pe *fs.PathError
		if _, err := list(filepath.Join(dir, "a")); !errors.As(err, &pe) {
			t.Errorf("%s(file) error = %v, want an *fs.PathError", name, err)
		}
	}
}
