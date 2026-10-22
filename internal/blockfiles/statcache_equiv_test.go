package blockfiles

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim (legacySysStamp per platform, in the _linux and
// _other files), and every case must decide as it did.

func legacyStatStamp(path string) (stamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return stamp{}, err
	}
	s := stamp{mtime: st.ModTime().UnixNano(), mode: uint32(st.Mode())}
	s.ctime, s.ino, s.dev = legacySysStamp(st)
	return s, nil
}

func legacyFileStamp(path string, err error) string {
	if err != nil {
		return ""
	}
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	ctime, ino, dev := legacySysStamp(st)
	return fmt.Sprintf("%s:%d:%d:%d:%d:%d", path, st.Size(), st.ModTime().UnixNano(), ctime, ino, dev)
}

func legacyCachePath(root string) string {
	switch strings.ToLower(pycompat.Strip(os.Getenv("CORPUS_NAV_FIND_INDEX"))) {
	case "off", "0", "no", "false":
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := filepath.Base(exe)
	if strings.HasSuffix(strings.TrimSuffix(name, ".exe"), ".test") && !cacheTestBinary {
		return ""
	}
	dir := join(join(root, "tools"), ".corpus-index")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return join(dir, "subblocks-"+name+".cache")
}

func legacyIsTestName(name string) bool {
	return strings.HasSuffix(strings.TrimSuffix(name, ".exe"), ".test")
}

func TestStampsMatchLegacy(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x-block1.md")
	if err := os.WriteFile(file, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	exe, exeErr := os.Executable()
	for _, p := range []string{dir, file, link, filepath.Join(dir, "missing"), exe} {
		got, err := statStamp(p)
		want, lerr := legacyStatStamp(p)
		if got != want || (err == nil) != (lerr == nil) {
			t.Errorf("statStamp(%s) = %+v, %v; legacy %+v, %v", p, got, err, want, lerr)
		}
		for _, e := range []error{nil, os.ErrNotExist} {
			if got, want := fileStamp(p, e), legacyFileStamp(p, e); got != want {
				t.Errorf("fileStamp(%s, %v) = %q, legacy %q", p, e, got, want)
			}
		}
	}
	if got, want := exeStamp(), legacyFileStamp(exe, exeErr); got != want || got == "" {
		t.Errorf("exeStamp = %q, legacy %q", got, want)
	}
}

func TestCachePathMatchesLegacy(t *testing.T) {
	withIndex := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withIndex, "tools", ".corpus-index"), 0o755); err != nil {
		t.Fatal(err)
	}
	fileIndex := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fileIndex, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fileIndex, "tools", ".corpus-index"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cacheTestBinary = false })
	for _, sw := range []string{"", "off", " OFF ", "\tno\n", "False", "0", "1", "on", "offline", "of f"} {
		for _, testBinary := range []bool{false, true} {
			for _, root := range []string{withIndex, fileIndex, t.TempDir(), filepath.Join(withIndex, "gone")} {
				t.Setenv("CORPUS_NAV_FIND_INDEX", sw)
				cacheTestBinary = testBinary
				if got, want := cachePath(root), legacyCachePath(root); got != want {
					t.Errorf("switch %q test %v root %s: %q, legacy %q", sw, testBinary, root, got, want)
				}
			}
		}
	}
	for _, name := range []string{"corpus-nav", "blockfiles.test", `C:\b\blockfiles.test.exe`, "x.test.exe", ".test", "test", "x.tests", "x.exe.test"} {
		if got, want := statcache.IsTestName(name), legacyIsTestName(filepath.Base(name)); got != want {
			t.Errorf("IsTestName(%q) = %v, legacy %v", name, got, want)
		}
	}
}

// TestSubCacheBytesMatchLegacy: the walk cache SubBlocks writes holds the legacy stamp
// of every walked directory, and its bytes are the ones the legacy stamps would write;
// the next call is served from it.
func TestSubCacheBytesMatchLegacy(t *testing.T) {
	root := cacheRoot(t, "a/x-block1.md", "a/b/c-bloque2.md", "os413-block3.md", "a/plain.md")
	if err := os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "y-block9.md")); err != nil {
		t.Fatal(err)
	}
	warm(t, root)
	got, err := os.ReadFile(cachePath(root))
	if err != nil {
		t.Fatal(err)
	}
	key, ok := cacheKey(root)
	if !ok {
		t.Fatal("no executable stamp")
	}
	bs, rec, err := parseSubCache(got, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.dirs) < 3 || len(rec.links) != 1 {
		t.Fatalf("record %d dirs, %d links; want >= 3 and 1", len(rec.dirs), len(rec.links))
	}
	legacy := &walkRecord{links: rec.links, dur: rec.dur}
	for _, d := range rec.dirs {
		st, err := legacyStatStamp(d.path)
		if err != nil || st != d.stamp {
			t.Errorf("%s: cached stamp %+v, legacy %+v, %v", d.path, d.stamp, st, err)
		}
		legacy.dirs = append(legacy.dirs, dirEntryStamp{d.path, st})
	}
	path := filepath.Join(t.TempDir(), "legacy.cache")
	if err := writeSubCache(path, key, bs, legacy); err != nil {
		t.Fatal(err)
	}
	if want, _ := os.ReadFile(path); !bytes.Equal(got, want) {
		t.Errorf("walk cache bytes differ from the legacy stamps'")
	}
}
