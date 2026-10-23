package modjar

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/statcache/stattest"
)

const member = "META-INF/module.xml"

type entry struct{ name, body string }

// writeJar writes a jar holding entries (stored or deflated as the zip writer picks).
func writeJar(t *testing.T, path string, entries ...entry) {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// corruptJar is a jar whose member has a bad CRC: it opens, but reading the member fails.
func corruptJar(t *testing.T, path string) {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.CreateRaw(&zip.FileHeader{Name: member, Method: zip.Store, CRC32: 1,
		CompressedSize64: 3, UncompressedSize64: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadMemberCases(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.jar")
	writeJar(t, ok, entry{"META-INF/MANIFEST.MF", "m"}, entry{"a/B.class", "x"},
		entry{member, "<module/>"}, entry{"META-INF/SIGN.RSA", "s"})
	twice := filepath.Join(dir, "twice.jar")
	writeJar(t, twice, entry{member, "first"}, entry{member, "last"})
	none := filepath.Join(dir, "none.jar")
	writeJar(t, none, entry{"a/B.class", "x"})
	bad := filepath.Join(dir, "bad.jar")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	crc := filepath.Join(dir, "crc.jar")
	corruptJar(t, crc)

	cases := []struct {
		name, path, data string
		meta             []string
		err              error
	}{
		{"member found", ok, "<module/>", []string{"META-INF/MANIFEST.MF", member, "META-INF/SIGN.RSA"}, nil},
		{"last duplicate wins", twice, "last", []string{member, member}, nil},
		{"no member", none, "", nil, ErrNoMember},
		{"not a zip", bad, "", nil, zip.ErrFormat},
		{"missing file", filepath.Join(dir, "gone.jar"), "", nil, os.ErrNotExist},
		{"corrupt member", crc, "", nil, zip.ErrChecksum},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := ReadMember(c.path, member)
			if r.Path != c.path {
				t.Errorf("Path = %q, want %q", r.Path, c.path)
			}
			if c.err == nil && r.Err != nil || c.err != nil && !errors.Is(r.Err, c.err) {
				t.Fatalf("Err = %v, want %v", r.Err, c.err)
			}
			if string(r.Data) != c.data {
				t.Errorf("Data = %q, want %q", r.Data, c.data)
			}
			if c.err == nil && !slices.Equal(r.MetaInf, c.meta) {
				t.Errorf("MetaInf = %q, want %q", r.MetaInf, c.meta)
			}
		})
	}
}

func TestReadAllKeepsInputOrderAndPerJarErrors(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := range 40 {
		p := filepath.Join(dir, fmt.Sprintf("j%02d.jar", i))
		switch i % 4 {
		case 1:
			writeJar(t, p, entry{"x", "y"}) // no member
		case 3:
			p = filepath.Join(dir, fmt.Sprintf("missing%02d.jar", i))
		default:
			writeJar(t, p, entry{member, fmt.Sprint("body", i)})
		}
		paths = append(paths, p)
	}
	for _, workers := range []int{0, 1, 3, 8, 100} {
		got := ReadAll(paths, member, workers)
		if len(got) != len(paths) {
			t.Fatalf("workers=%d: %d results for %d paths", workers, len(got), len(paths))
		}
		for i, r := range got {
			if r.Path != paths[i] {
				t.Fatalf("workers=%d: result %d is %s, want %s", workers, i, r.Path, paths[i])
			}
			switch i % 4 {
			case 1:
				if !errors.Is(r.Err, ErrNoMember) {
					t.Errorf("workers=%d %d: Err = %v, want ErrNoMember", workers, i, r.Err)
				}
			case 3:
				if !errors.Is(r.Err, os.ErrNotExist) {
					t.Errorf("workers=%d %d: Err = %v, want not-exist", workers, i, r.Err)
				}
			default:
				if r.Err != nil || string(r.Data) != fmt.Sprint("body", i) {
					t.Errorf("workers=%d %d: %q, %v", workers, i, r.Data, r.Err)
				}
			}
		}
	}
	if got := ReadAll(nil, member, 8); len(got) != 0 {
		t.Errorf("ReadAll(nil) = %v", got)
	}
}

// opened counts the jars a cached read actually opened.
func opened(t *testing.T, fn func()) int64 {
	t.Helper()
	before := opens.Load()
	fn()
	return opens.Load() - before
}

func TestCacheHitMissAndInvalidation(t *testing.T) {
	dir, cacheDir := t.TempDir(), t.TempDir()
	a, b, none := filepath.Join(dir, "a.jar"), filepath.Join(dir, "b.jar"), filepath.Join(dir, "none.jar")
	writeJar(t, a, entry{member, "A1"}, entry{"META-INF/X.RSA", ""})
	writeJar(t, b, entry{member, "B1"})
	writeJar(t, none, entry{"x", "y"})
	gone := filepath.Join(dir, "gone.jar")
	paths := []string{a, b, none, gone}

	read := func() []Result {
		c := OpenCache(cacheDir, member)
		res := c.ReadAll(paths, 2)
		if err := c.Save(); err != nil {
			t.Fatal(err)
		}
		return res
	}
	var cold, warm []Result
	if n := opened(t, func() { cold = read() }); n != 4 {
		t.Errorf("cold read opened %d jars, want 4", n)
	}
	// The missing jar is never cached (an open error may not repeat); the no-member jar is.
	if n := opened(t, func() { warm = read() }); n != 1 {
		t.Errorf("warm read opened %d jars, want 1 (the missing one)", n)
	}
	for i := range cold {
		if string(cold[i].Data) != string(warm[i].Data) || !slices.Equal(cold[i].MetaInf, warm[i].MetaInf) ||
			(cold[i].Err == nil) != (warm[i].Err == nil) || cold[i].Path != warm[i].Path {
			t.Errorf("result %d: cold %+v, warm %+v", i, cold[i], warm[i])
		}
	}
	if !errors.Is(warm[2].Err, ErrNoMember) || !errors.Is(warm[3].Err, os.ErrNotExist) {
		t.Errorf("warm errors: %v, %v", warm[2].Err, warm[3].Err)
	}

	// Same size, new mtime: read again.
	writeJar(t, a, entry{member, "A2"}, entry{"META-INF/X.RSA", ""})
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(a, later, later); err != nil {
		t.Fatal(err)
	}
	var res []Result
	if n := opened(t, func() { res = read() }); n != 2 {
		t.Errorf("after an mtime change opened %d jars, want 2", n)
	}
	if string(res[0].Data) != "A2" {
		t.Errorf("stale cache: %q", res[0].Data)
	}

	// Another member is another cache.
	other := OpenCache(cacheDir, "META-INF/other.xml")
	if n := opened(t, func() { other.ReadAll([]string{a}, 1) }); n != 1 {
		t.Errorf("another member opened %d jars, want 1", n)
	}

	// A corrupt cache file is ignored, never trusted.
	files, _ := filepath.Glob(filepath.Join(cacheDir, "*"))
	for _, f := range files {
		if err := os.WriteFile(f, []byte("garbage"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if n := opened(t, func() { res = read() }); n != 4 {
		t.Errorf("after a corrupt cache opened %d jars, want 4", n)
	}
	if string(res[1].Data) != "B1" {
		t.Errorf("after a corrupt cache: %q", res[1].Data)
	}

	// A jar gone from a directory read again is dropped from the file.
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	c := OpenCache(cacheDir, member)
	c.ReadAll([]string{a, none}, 2)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if c = OpenCache(cacheDir, member); len(c.entries) != 2 {
		t.Errorf("after removing b.jar the cache holds %d entries, want 2", len(c.entries))
	}
	if _, ok := c.entries[b]; ok {
		t.Errorf("the cache still holds the removed %s", b)
	}
}

func TestDisabledCacheReadsThroughAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jar")
	writeJar(t, a, entry{member, "A"})
	c := OpenCache("", member)
	for range 2 {
		if n := opened(t, func() { c.ReadAll([]string{a}, 1) }); n != 1 {
			t.Errorf("disabled cache opened %d jars, want 1", n)
		}
	}
	if err := c.Save(); err != nil {
		t.Errorf("Save on a disabled cache: %v", err)
	}
}

func TestDefaultDirFollowsXDG(t *testing.T) {
	t.Setenv(envCache, "")
	t.Setenv("XDG_CACHE_HOME", "/x/cache")
	if got, want := defaultDir(false), filepath.Join("/x/cache", "niagara-go", "modjar"); got != want {
		t.Errorf("defaultDir = %q, want %q", got, want)
	}
	if got := defaultDir(true); got != "" {
		t.Errorf("a test binary gets %q, want no cache", got)
	}
	for _, off := range []string{"off", "0", "no", "false", " OFF "} {
		t.Setenv(envCache, off)
		if got := defaultDir(false); got != "" {
			t.Errorf("%s=%q: defaultDir = %q, want no cache", envCache, off, got)
		}
	}
}

// TestCacheMissesAnEditThatKeepsSizeAndMtime: a jar rewritten with the same size and
// modification time (cp -p, rsync -a, tar x, touch -r) is read again, never answered
// from the cache.
func TestCacheMissesAnEditThatKeepsSizeAndMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime and inode are read on Linux only (statcache.Sys)")
	}
	dir, cacheDir := t.TempDir(), t.TempDir()
	a, b := filepath.Join(dir, "a.jar"), filepath.Join(dir, "b.jar")
	writeJar(t, a, entry{member, "A1"})
	writeJar(t, b, entry{member, "A2"}) // same size, other content
	read := func() string {
		c := OpenCache(cacheDir, member)
		res := c.ReadAll([]string{a}, 1)
		if err := c.Save(); err != nil {
			t.Fatal(err)
		}
		return string(res[0].Data)
	}
	if got := read(); got != "A1" {
		t.Fatalf("cold read %q", got)
	}
	edited, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	stattest.EditKeepingStamp(t, a, edited)
	if got := read(); got != "A2" {
		t.Errorf("stale cache after an edit keeping size and mtime: %q, want A2", got)
	}
}
