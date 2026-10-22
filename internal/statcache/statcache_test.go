package statcache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// demoKey and demoFile are a typical caller's cache: a format version plus the
// stamps of a file set, the payload stored under that key.
type demoKey struct {
	Format int
	Files  []Stamp
}

type demoFile struct {
	Key     demoKey
	Payload string
}

func (k demoKey) equal(o demoKey) bool {
	if k.Format != o.Format || len(k.Files) != len(o.Files) {
		return false
	}
	for i := range k.Files {
		if k.Files[i] != o.Files[i] {
			return false
		}
	}
	return true
}

// lookup is the caller flow: key the file set, decode the cache, compare.
func lookup(cache string, format int, paths []string) (string, bool) {
	key := demoKey{format, Files(paths, 1)}
	var cf demoFile
	if !ReadGob(cache, &cf) || !cf.Key.equal(key) {
		return "", false
	}
	return cf.Payload, true
}

func store(t *testing.T, cache string, format int, paths []string, payload string) {
	t.Helper()
	if !WriteGob(cache, &demoFile{demoKey{format, Files(paths, 1)}, payload}) {
		t.Fatalf("WriteGob %s failed", cache)
	}
}

func writeFile(t *testing.T, path, text string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestCacheFlow(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name   string
		change func(t *testing.T, dir string, paths *[]string, cache string)
		format int
		hit    bool
	}{
		{"hit", func(*testing.T, string, *[]string, string) {}, 1, true},
		{"miss without cache", func(t *testing.T, _ string, _ *[]string, cache string) {
			if err := os.Remove(cache); err != nil {
				t.Fatal(err)
			}
		}, 1, false},
		{"stale size", func(t *testing.T, dir string, _ *[]string, _ string) {
			writeFile(t, filepath.Join(dir, "a"), "longer", base)
		}, 1, false},
		{"stale mtime", func(t *testing.T, dir string, _ *[]string, _ string) {
			writeFile(t, filepath.Join(dir, "a"), "aa", base.Add(time.Nanosecond))
		}, 1, false},
		{"file added", func(t *testing.T, dir string, paths *[]string, _ string) {
			c := filepath.Join(dir, "c")
			writeFile(t, c, "c", base)
			*paths = append(*paths, c)
		}, 1, false},
		{"file removed", func(t *testing.T, dir string, _ *[]string, _ string) {
			if err := os.Remove(filepath.Join(dir, "b")); err != nil {
				t.Fatal(err)
			}
		}, 1, false},
		{"corrupt file", func(t *testing.T, _ string, _ *[]string, cache string) {
			if err := os.WriteFile(cache, []byte("not a gob stream"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, 1, false},
		{"version bump", func(*testing.T, string, *[]string, string) {}, 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			paths := []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
			writeFile(t, paths[0], "aa", base)
			writeFile(t, paths[1], "bb", base)
			cache := filepath.Join(dir, "sub", "cache.gob")
			store(t, cache, 1, paths, "payload")
			tc.change(t, dir, &paths, cache)
			got, hit := lookup(cache, tc.format, paths)
			if hit != tc.hit || (hit && got != "payload") {
				t.Fatalf("lookup = %q, %v; want hit %v", got, hit, tc.hit)
			}
		})
	}
}

func TestWriteGobIsPrivateAndLeavesNothingOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.gob")
	if !WriteGob(path, map[string]int{"a": 1}) {
		t.Fatal("WriteGob failed")
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v; want 0600", st.Mode(), err)
	}
	bad := filepath.Join(dir, "bad.gob")
	if WriteGob(bad, make(chan int)) { // a chan cannot be gob-encoded
		t.Fatal("WriteGob of a chan succeeded")
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("dir holds %d entries after a failed write, want 1", len(ents))
	}
}

func TestFilesMarksMissingAndKeepsOrderInParallel(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := range 9 {
		p := filepath.Join(dir, string(rune('a'+i)))
		writeFile(t, p, string(make([]byte, i)), time.Unix(int64(1000+i), 0))
		paths = append(paths, p)
	}
	paths = append(paths, filepath.Join(dir, "missing"))
	seq, par := Files(paths, 1), Files(paths, 4)
	for i := range paths {
		if seq[i] != par[i] {
			t.Fatalf("stamp %d: sequential %v, parallel %v", i, seq[i], par[i])
		}
	}
	for i := range 9 {
		if want := (Stamp{int64(i), int64(1000+i) * 1e9}); seq[i] != want {
			t.Fatalf("stamp %d = %v, want %v", i, seq[i], want)
		}
	}
	if seq[9] != Missing || Missing.Exists() || !seq[0].Exists() {
		t.Fatalf("missing stamp = %v", seq[9])
	}
}

func TestRegularAndSame(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	writeFile(t, p, "12345", time.Unix(5000, 0))
	if _, ok := Regular(dir, 0); ok {
		t.Fatal("a directory is not a regular file")
	}
	if _, ok := Regular(p, 6); ok {
		t.Fatal("a file under minSize is not cacheable")
	}
	s, ok := Regular(p, 5)
	if !ok || s != (Stamp{5, 5000e9}) {
		t.Fatalf("Regular = %v, %v", s, ok)
	}
	if !Same(p, s) {
		t.Fatal("an unchanged file is not Same")
	}
	// The error-not-cached rule: a result computed while the file changed is not stored.
	writeFile(t, p, "123456", time.Unix(5000, 0))
	if Same(p, s) {
		t.Fatal("a resized file is Same")
	}
	if Same(filepath.Join(dir, "gone"), s) {
		t.Fatal("a missing file is Same")
	}
}

func TestOff(t *testing.T) {
	for _, v := range []string{"off", " OFF ", "0", "no", "False"} {
		t.Setenv("STATCACHE_TEST_SWITCH", v)
		if !Off("STATCACHE_TEST_SWITCH") {
			t.Errorf("%q does not turn the cache off", v)
		}
	}
	for _, v := range []string{"", "on", "1", "offline"} {
		t.Setenv("STATCACHE_TEST_SWITCH", v)
		if Off("STATCACHE_TEST_SWITCH") {
			t.Errorf("%q turns the cache off", v)
		}
	}
}

func TestUserDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/x/cache")
	if got := UserDir("niagara-go", "modjar"); got != "/x/cache/niagara-go/modjar" {
		t.Fatalf("UserDir = %q", got)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got := UserDir("niagara-go", "bog-nav"); got != "/home/u/.cache/niagara-go/bog-nav" {
		t.Fatalf("UserDir = %q", got)
	}
}

func TestTestBinary(t *testing.T) {
	if !TestBinary() {
		t.Fatal("a go test binary is not seen as one")
	}
}

func TestSysOnLinux(t *testing.T) {
	st, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctime, ino, _, ok := Sys(st)
	if runtime.GOOS == "linux" && (!ok || ctime == 0 || ino == 0) {
		t.Fatalf("Sys = %d, %d, %v on Linux", ctime, ino, ok)
	}
	if runtime.GOOS != "linux" && ok {
		t.Fatal("Sys reports values off Linux")
	}
}
