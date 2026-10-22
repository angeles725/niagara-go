package blockfiles

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// cacheRoot is a corpus root with the find-cache directory in place and caching enabled
// for this test binary.
func cacheRoot(t *testing.T, files ...string) string {
	t.Helper()
	cacheTestBinary = true
	t.Cleanup(func() { cacheTestBinary = false })
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tools", ".corpus-index"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, files)
	return root
}

// backdate moves the mtime of every directory under root an hour back, so that a later
// change in the same clock tick still changes the stamp (the racy-stamp guard is the
// production answer to that; the tests take it out of the way deterministically).
func backdate(t *testing.T, root string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			return os.Chtimes(p, old, old)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// trust dates the cache file an hour ahead so no recorded stamp is racy against it.
func trust(t *testing.T, root string) {
	t.Helper()
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(cachePath(root), future, future); err != nil {
		t.Fatal(err)
	}
}

// warm builds the cache of root, then checks that the next call is served from it
// without a walk; it returns that result.
func warm(t *testing.T, root string) []Block {
	t.Helper()
	backdate(t, root)
	first := mustSub(root)
	trust(t, root)
	before := walks.Load()
	again := mustSub(root)
	if n := walks.Load() - before; n != 0 {
		t.Fatalf("warm call walked %d times, want 0 (served from the cache)", n)
	}
	if !reflect.DeepEqual(again, first) {
		t.Fatalf("cached result %q differs from the walk %q", again, first)
	}
	return first
}

// uncached is SubBlocks without any cache.
func uncached(root string) []Block { bs, _, _ := walkSubBlocks(root, false); return bs }

func TestSubBlocksCacheServesWarmCalls(t *testing.T) {
	root := cacheRoot(t, "a/x-block1.md", "a/b/c-bloque2.md", "os413-block3.md", "a/plain.md")
	got := warm(t, root)
	if want := uncached(root); !reflect.DeepEqual(got, want) || len(want) != 3 {
		t.Fatalf("SubBlocks = %q, want %q", got, want)
	}
}

// TestSubBlocksCacheNeverStale changes the tree after the cache was built and trusted:
// the next call must see the change (a full walk), with output identical to an uncached
// walk.
func TestSubBlocksCacheNeverStale(t *testing.T) {
	posix := runtime.GOOS != "windows"
	outside := t.TempDir() // a symlink target no walked directory stamp covers
	cases := []struct {
		name   string
		posix  bool // needs symlinks or POSIX modes
		setup  func(t *testing.T, root string)
		mutate func(t *testing.T, root string)
	}{
		{"add a file at the root", false, nil, func(t *testing.T, root string) {
			writeTree(t, root, []string{"new-block9.md"})
		}},
		{"add a nested file", false, nil, func(t *testing.T, root string) {
			writeTree(t, root, []string{"a/b/n-block8.md"})
		}},
		{"add a directory holding a block", false, nil, func(t *testing.T, root string) {
			writeTree(t, root, []string{"a/new/deep/d-block7.md"})
		}},
		{"remove a file", false, nil, func(t *testing.T, root string) {
			must(t, os.Remove(filepath.Join(root, "a/b/c-bloque2.md")))
		}},
		{"rename a file", false, nil, func(t *testing.T, root string) {
			must(t, os.Rename(filepath.Join(root, "a/x-block1.md"), filepath.Join(root, "a/x-block5.md")))
		}},
		{"rename a file into a block name", false, nil, func(t *testing.T, root string) {
			must(t, os.Rename(filepath.Join(root, "a/plain.md"), filepath.Join(root, "a/plain-block4.md")))
		}},
		{"rename a directory", false, nil, func(t *testing.T, root string) {
			must(t, os.Rename(filepath.Join(root, "a/b"), filepath.Join(root, "a/q")))
		}},
		{"rename a directory into a skipped name", false, nil, func(t *testing.T, root string) {
			must(t, os.Rename(filepath.Join(root, "a"), filepath.Join(root, "_backups")))
		}},
		{"remove a directory", false, nil, func(t *testing.T, root string) {
			must(t, os.RemoveAll(filepath.Join(root, "a/b")))
		}},
		{"make an unreadable directory readable", true, func(t *testing.T, root string) {
			writeTree(t, root, []string{"locked/l-block6.md"})
			must(t, os.Chmod(filepath.Join(root, "locked"), 0))
			t.Cleanup(func() { os.Chmod(filepath.Join(root, "locked"), 0o755) })
		}, func(t *testing.T, root string) {
			must(t, os.Chmod(filepath.Join(root, "locked"), 0o755))
		}},
		{"make a directory unreadable", true, nil, func(t *testing.T, root string) {
			must(t, os.Chmod(filepath.Join(root, "a/b"), 0))
			t.Cleanup(func() { os.Chmod(filepath.Join(root, "a/b"), 0o755) })
		}},
		{"a block symlink's target becomes a directory", true, func(t *testing.T, root string) {
			writeTree(t, outside, []string{"tgt.md"})
			must(t, os.Symlink(filepath.Join(outside, "tgt.md"), filepath.Join(root, "a/link-block3.md")))
		}, func(t *testing.T, root string) {
			must(t, os.Remove(filepath.Join(outside, "tgt.md")))
			must(t, os.Mkdir(filepath.Join(outside, "tgt.md"), 0o755))
		}},
		{"a block symlink's dangling target appears as a directory", true, func(t *testing.T, root string) {
			must(t, os.Symlink(filepath.Join(outside, "later"), filepath.Join(root, "a/dang-block2.md")))
		}, func(t *testing.T, root string) {
			must(t, os.Mkdir(filepath.Join(outside, "later"), 0o755))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.posix && (!posix || os.Geteuid() == 0) {
				t.Skip("needs POSIX symlinks and modes, as a non-root user")
			}
			root := cacheRoot(t, "a/x-block1.md", "a/b/c-bloque2.md", "a/plain.md", "os413-block3.md")
			if tc.setup != nil {
				tc.setup(t, root)
			}
			before := warm(t, root)
			tc.mutate(t, root)
			got, want := mustSub(root), uncached(root)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("after the change SubBlocks = %q, want %q", got, want)
			}
			if reflect.DeepEqual(got, before) {
				t.Fatalf("the change left the result %q unchanged: the case proves nothing", got)
			}
		})
	}
}

// TestSubBlocksCacheContentEditKeepsResult: SubBlocks reads names only, so a content
// edit neither changes the result nor costs a walk.
func TestSubBlocksCacheContentEditKeepsResult(t *testing.T) {
	root := cacheRoot(t, "a/x-block1.md")
	before := warm(t, root)
	must(t, os.WriteFile(filepath.Join(root, "a/x-block1.md"), []byte("# other\n"), 0o644))
	n := walks.Load()
	if got := mustSub(root); !reflect.DeepEqual(got, before) || walks.Load() != n {
		t.Fatalf("content edit: got %q (walks +%d), want %q from the cache", got, walks.Load()-n, before)
	}
}

// TestSubBlocksCacheFallsBack: a corrupt, foreign or unusable cache is a full walk with
// the uncached output.
func TestSubBlocksCacheFallsBack(t *testing.T) {
	files := []string{"a/x-block1.md", "a/b/c-bloque2.md"}
	damage := map[string]func(t *testing.T, root string){
		"garbage":   func(t *testing.T, root string) { overwrite(t, root, []byte("not a cache")) },
		"empty":     func(t *testing.T, root string) { overwrite(t, root, nil) },
		"truncated": func(t *testing.T, root string) { edit(t, root, func(b []byte) []byte { return b[:len(b)/2] }) },
		"bit flip": func(t *testing.T, root string) {
			edit(t, root, func(b []byte) []byte { b[len(b)-1] ^= 0x20; return b }) // the last corpus name: still parses
		},
		"extra bytes": func(t *testing.T, root string) { edit(t, root, func(b []byte) []byte { return append(b, 0) }) },
		"another executable": func(t *testing.T, root string) {
			prev := exeStamp
			exeStamp = func() string { return prev() + "-rebuilt" }
			t.Cleanup(func() { exeStamp = prev })
		},
		"another root's cache": func(t *testing.T, root string) {
			other := cacheRoot(t, "z/z-block9.md")
			warm(t, other)
			data, err := os.ReadFile(cachePath(other))
			must(t, err)
			overwrite(t, root, data)
		},
	}
	for name, hurt := range damage {
		t.Run(name, func(t *testing.T) {
			root := cacheRoot(t, files...)
			warm(t, root)
			hurt(t, root)
			n := walks.Load()
			if got, want := mustSub(root), uncached(root); !reflect.DeepEqual(got, want) {
				t.Fatalf("SubBlocks = %q, want %q", got, want)
			}
			if walks.Load() == n {
				t.Fatal("a damaged cache was served without a walk")
			}
		})
	}
}

func TestSubBlocksCacheUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a read-only directory, as a non-root user")
	}
	root := cacheRoot(t, "a/x-block1.md")
	dir := filepath.Dir(cachePath(root))
	must(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if got, want := mustSub(root), uncached(root); !reflect.DeepEqual(got, want) || len(got) != 1 {
		t.Fatalf("SubBlocks = %q, want %q", got, want)
	}
	if _, err := os.Stat(cachePath(root)); !os.IsNotExist(err) {
		t.Fatalf("cache written into a read-only directory: %v", err)
	}
}

// TestSubBlocksCacheOnlyWhereEnabled: no cache without the find-cache directory, with
// CORPUS_NAV_FIND_INDEX off, or from a test binary that did not opt in.
func TestSubBlocksCacheOnlyWhereEnabled(t *testing.T) {
	t.Run("no index directory", func(t *testing.T) {
		root := cacheRoot(t, "a/x-block1.md")
		must(t, os.RemoveAll(filepath.Join(root, "tools")))
		mustSub(root)
		if _, err := os.Stat(filepath.Join(root, "tools")); !os.IsNotExist(err) {
			t.Fatalf("SubBlocks created the index directory: %v", err)
		}
	})
	for name, prep := range map[string]func(t *testing.T){
		"CORPUS_NAV_FIND_INDEX=off": func(t *testing.T) { t.Setenv("CORPUS_NAV_FIND_INDEX", " Off ") },
		"test binary":               func(t *testing.T) { cacheTestBinary = false },
	} {
		t.Run(name, func(t *testing.T) {
			root := cacheRoot(t, "a/x-block1.md")
			prep(t)
			if len(mustSub(root)) != 1 {
				t.Fatal("SubBlocks lost the block")
			}
			if _, err := os.Stat(cachePath(root)); !os.IsNotExist(err) {
				t.Fatalf("cache written: %v", err)
			}
		})
	}
}

// TestSubBlocksCacheRacyStamp: a directory stamp too close to the cache file's own time
// may hide a change made in the same clock tick, so that cache is not trusted.
func TestSubBlocksCacheRacyStamp(t *testing.T) {
	root := cacheRoot(t, "a/x-block1.md")
	mustSub(root) // the tree was just written: its stamps are racy
	if _, err := os.Stat(cachePath(root)); err != nil {
		t.Fatalf("no cache written: %v", err)
	}
	n := walks.Load()
	mustSub(root)
	if walks.Load() == n {
		t.Fatal("a cache with racy stamps was trusted")
	}
}

// TestSubBlocksCacheSkipsInvalidWalk: a walk that hit an error a later walk may not repeat
// (here a block-named symlink loop, ELOOP, neither missing nor unreadable) writes no cache,
// so the next call walks again.
func TestSubBlocksCacheSkipsInvalidWalk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks")
	}
	root := cacheRoot(t, "a/x-block1.md")
	must(t, os.Symlink("y-block3.md", filepath.Join(root, "a", "w-block2.md")))
	must(t, os.Symlink("w-block2.md", filepath.Join(root, "a", "y-block3.md")))
	backdate(t, root)
	want := uncached(root)
	if got := mustSub(root); !reflect.DeepEqual(got, want) {
		t.Fatalf("SubBlocks = %q, want %q", got, want)
	}
	if _, err := os.Stat(cachePath(root)); !os.IsNotExist(err) {
		t.Fatalf("an invalid walk was cached (stat: %v)", err)
	}
	n := walks.Load()
	if got := mustSub(root); !reflect.DeepEqual(got, want) || walks.Load() != n+1 {
		t.Fatalf("second call: got %q (walks +%d), want %q from one new walk", got, walks.Load()-n, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func overwrite(t *testing.T, root string, data []byte) {
	t.Helper()
	must(t, os.WriteFile(cachePath(root), data, 0o600))
	trust(t, root)
}

func edit(t *testing.T, root string, f func([]byte) []byte) {
	t.Helper()
	data, err := os.ReadFile(cachePath(root))
	must(t, err)
	overwrite(t, root, f(data))
}

// mustSub is SubBlocks on a test tree whose root can be listed.
func mustSub(root string) []Block {
	bs, err := SubBlocks(root)
	if err != nil {
		panic(err)
	}
	return bs
}
