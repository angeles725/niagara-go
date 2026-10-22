package pycompat

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// walkTree builds a tree of empty files (a trailing "/" makes a directory) under a temp dir.
func walkTree(t *testing.T, paths ...string) string {
	t.Helper()
	top := t.TempDir()
	for _, p := range paths {
		full := filepath.Join(top, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return top
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symlinks here: %v", err) // Windows without the privilege
	}
}

// Walk lists a symlinked folder among the folders without entering it, a broken symlink among
// the files, and descends only what visit returns, in sorted order.
func TestWalkSortedPrunesAndSkipsDirSymlinks(t *testing.T) {
	top := walkTree(t, "a/x", "a/skip/y", "f", "real/z")
	symlinkOrSkip(t, filepath.Join(top, "real"), filepath.Join(top, "a", "link"))
	symlinkOrSkip(t, filepath.Join(top, "missing"), filepath.Join(top, "broken"))
	var got []string
	Walk(top, func(dir string, dirs, files []string) []string {
		rel, _ := filepath.Rel(top, dir)
		got = append(got, rel+" "+strings.Join(dirs, ",")+" | "+strings.Join(files, ","))
		var keep []string
		for _, d := range dirs {
			if d != "skip" {
				keep = append(keep, d)
			}
		}
		return keep
	})
	want := []string{". a,real | broken,f", "a link,skip | x", "real  | z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Walk:\n got %q\nwant %q", got, want)
	}
}

// Paths are joined like os.path.join, so a top ending in a separator keeps its spelling.
func TestWalkJoinsLikeOsPathJoin(t *testing.T) {
	top := walkTree(t, "b/F") + string(filepath.Separator)
	var dirs []string
	Walk(top, func(dir string, sub, _ []string) []string {
		dirs = append(dirs, dir)
		return sub
	})
	if want := []string{top, top + "b"}; !reflect.DeepEqual(dirs, want) {
		t.Errorf("dirs %q, want %q", dirs, want)
	}
}

// Unsorted lists each directory in os.scandir's order: the same entries as Walk, in the order
// Listdir returns them.
func TestWalkWithUnsortedFollowsDirectoryOrder(t *testing.T) {
	top := walkTree(t, "c", "a", "e", "b/", "d/")
	names, err := Listdir(top)
	if err != nil {
		t.Fatal(err)
	}
	var wantDirs, wantFiles []string
	for _, n := range names {
		if n == "b" || n == "d" {
			wantDirs = append(wantDirs, n)
		} else {
			wantFiles = append(wantFiles, n)
		}
	}
	var gotDirs, gotFiles []string
	WalkWith(top, WalkOptions{Unsorted: true}, func(dir string, dirs, files []string) []string {
		if dir == top {
			gotDirs, gotFiles = dirs, files
		}
		return dirs
	})
	if !reflect.DeepEqual(gotDirs, wantDirs) || !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Errorf("unsorted top: dirs %q files %q, want %q %q", gotDirs, gotFiles, wantDirs, wantFiles)
	}
	sorted := append([]string(nil), gotFiles...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(sorted, []string{"a", "c", "e"}) {
		t.Errorf("unsorted files %q", gotFiles)
	}
}

// OnError is os.walk's onerror: it gets the directory that cannot be read and the error, and the
// walk goes on without it; without OnError the failure is silent.
func TestWalkWithOnError(t *testing.T) {
	top := walkTree(t, "a/x")
	missing := filepath.Join(top, "gone")
	var visited, failed []string
	visit := func(dir string, dirs, _ []string) []string {
		visited = append(visited, dir)
		return append(dirs, "gone")
	}
	WalkWith(top, WalkOptions{OnError: func(dir string, err error) {
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("onerror(%q) err %v", dir, err)
		}
		failed = append(failed, dir)
	}}, visit)
	if want := []string{filepath.Join(top, "a", "gone"), missing}; !reflect.DeepEqual(failed, want) {
		t.Errorf("onerror dirs %q, want %q", failed, want)
	}
	if want := []string{top, filepath.Join(top, "a")}; !reflect.DeepEqual(visited, want) {
		t.Errorf("visited %q, want %q", visited, want)
	}
	visited = nil
	Walk(missing, visit)
	if visited != nil {
		t.Errorf("Walk of a missing top visited %q", visited)
	}
}
