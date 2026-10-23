package bognav

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/statcache/stattest"
)

// The parsed type catalog is cached per decompiler dir under $XDG_CACHE_HOME/niagara-go/bog-nav,
// keyed by the set of the dir's .java files and their statcache.Stamp.

const writableRel = "control/control-rt/vineflower/javax/baja/control/BBooleanWritable.java"

// cacheEnv points the catalog cache at a fresh directory and returns it.
func cacheEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("NIAGARA_CACHE_DIR", "")
	os.Unsetenv("NIAGARA_CACHE_DIR")
	return dir
}

// miniRoot copies testdata/organized-mini into a temporary organized/ root.
func miniRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "organized")
	src := filepath.Join("testdata", "organized-mini")
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(root, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// freshCatalog is a catalog over root with nothing loaded, as a new process has.
func freshCatalog(root string) *typeCatalog {
	return &typeCatalog{root: root, classes: map[string]*classRec{}, byModule: map[string]map[string]bool{},
		dirKeys: map[string]map[string]bool{}}
}

func fallbackDefault(t *testing.T, root string) (string, string) {
	t.Helper()
	key, props := freshCatalog(root).defaults("control", "BooleanWritable")
	rec, _ := props.Get("fallback")
	return key, rec.Default
}

func cacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestCatalogCacheWrittenUnderXDGCacheHome(t *testing.T) {
	cache := cacheEnv(t)
	root := miniRoot(t)
	if key, _ := fallbackDefault(t, root); key == "" {
		t.Fatal("BooleanWritable did not resolve")
	}
	files := cacheFiles(t, cache)
	if len(files) == 0 {
		t.Fatal("no catalog cache written under XDG_CACHE_HOME")
	}
	for _, f := range files {
		if !strings.HasPrefix(f, filepath.Join(cache, "niagara-go", "bog-nav")+string(filepath.Separator)) {
			t.Errorf("cache file %s outside niagara-go/bog-nav", f)
		}
	}
	for _, f := range cacheFiles(t, filepath.Dir(root)) {
		if !strings.HasSuffix(f, ".java") && !strings.HasSuffix(f, ".md") {
			t.Errorf("cache artifact %s written inside the organized tree", f)
		}
	}
}

func TestCatalogCacheServesUnchangedSources(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a mode-000 file stays readable here")
	}
	cacheEnv(t)
	root := miniRoot(t)
	key, want := fallbackDefault(t, root)
	if key == "" {
		t.Fatal("BooleanWritable did not resolve")
	}
	// Same size, mtime and file set but unreadable: only the cache can still answer.
	path := filepath.Join(root, writableRel)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	gotKey, got := fallbackDefault(t, root)
	if gotKey != key || got != want {
		t.Fatalf("second run = %q %q, want the cached %q %q", gotKey, got, key, want)
	}
}

func TestCatalogCacheInvalidatesOnSourceChange(t *testing.T) {
	cacheEnv(t)
	root := miniRoot(t)
	path := filepath.Join(root, writableRel)
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := "fallback = newProperty(0, new BStatusBoolean(false, BStatus.nullStatus), null)"
	if !strings.Contains(string(orig), old) {
		t.Fatal("fixture changed: fallback declaration not found")
	}
	if _, d := fallbackDefault(t, root); !strings.Contains(d, "false") {
		t.Fatalf("fallback default %q", d)
	}
	stamp := time.Now().Add(-time.Hour)
	write := func(text string, mtime time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	// A size change.
	write(strings.Replace(string(orig), old, strings.Replace(old, "false", "true ", 1), 1), stamp)
	if _, d := fallbackDefault(t, root); !strings.Contains(d, "true") {
		t.Fatalf("after a size change fallback default = %q, want the new true", d)
	}
	// The same size, only the mtime differs.
	write(strings.Replace(string(orig), old, strings.Replace(old, "false", "FALSE", 1), 1), stamp.Add(time.Minute))
	if _, d := fallbackDefault(t, root); !strings.Contains(d, "FALSE") {
		t.Fatalf("after an mtime change fallback default = %q, want the new FALSE", d)
	}
	// A new file in the dir.
	extra := filepath.Join(filepath.Dir(path), "BExtraPoint.java")
	if err := os.WriteFile(extra, []byte("package javax.baja.control;\npublic class BExtraPoint extends BBooleanPoint {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if key := freshCatalog(root).resolveType("control", "ExtraPoint"); key != "javax.baja.control.BExtraPoint" {
		t.Fatalf("after adding a file resolveType = %q", key)
	}
	// A removed file.
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if key := freshCatalog(root).resolveType("control", "ExtraPoint"); key != "" {
		t.Fatalf("after removing a file resolveType = %q, want none", key)
	}
}

func TestCatalogCacheOff(t *testing.T) {
	cache := cacheEnv(t)
	t.Setenv("NIAGARA_CACHE_DIR", "off")
	if key, _ := fallbackDefault(t, miniRoot(t)); key == "" {
		t.Fatal("BooleanWritable did not resolve")
	}
	if files := cacheFiles(t, cache); len(files) != 0 {
		t.Fatalf("NIAGARA_CACHE_DIR=off still wrote %v", files)
	}
}

// TestCatalogCacheMissesAnEditThatKeepsSizeAndMtime: a source rewritten with the same
// size and modification time (cp -p, rsync -a, tar x, touch -r) changes its
// statcache.Stamp (ctime, inode), so it is parsed again, never answered from the cache.
func TestCatalogCacheMissesAnEditThatKeepsSizeAndMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime and inode are read on Linux only (statcache.Sys)")
	}
	cacheEnv(t)
	root := miniRoot(t)
	path := filepath.Join(root, writableRel)
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := "fallback = newProperty(0, new BStatusBoolean(false, BStatus.nullStatus), null)"
	if !strings.Contains(string(orig), old) {
		t.Fatal("fixture changed: fallback declaration not found")
	}
	if _, d := fallbackDefault(t, root); !strings.Contains(d, "false") {
		t.Fatalf("fallback default %q", d)
	}
	edit := strings.Replace(string(orig), old, strings.Replace(old, "false", "FALSE", 1), 1)
	stattest.EditKeepingStamp(t, path, []byte(edit))
	if _, d := fallbackDefault(t, root); !strings.Contains(d, "FALSE") {
		t.Fatalf("after an edit keeping size and mtime fallback default = %q, want the new FALSE", d)
	}
}
