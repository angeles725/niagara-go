package statcache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/statcache/stattest"
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
			writeFile(t, filepath.Join(dir, "a"), "aa", base.Add(time.Second))
		}, 1, false},
		{"stale mtime by 1ns", func(t *testing.T, dir string, _ *[]string, _ string) {
			p := filepath.Join(dir, "a")
			writeFile(t, p, "aa", base.Add(time.Nanosecond))
			if Stat(p).MtimeNs == base.UnixNano() {
				t.Skip("the file system does not keep nanoseconds")
			}
		}, 1, false},
		{"rewritten with the same size and mtime", func(t *testing.T, dir string, _ *[]string, _ string) {
			stattest.EditKeepingStamp(t, filepath.Join(dir, "a"), []byte("zz")) // the ctime still changes
		}, 1, runtime.GOOS != "linux"}, // off Linux a Stamp is size and mtime only
		{"files reordered", func(_ *testing.T, _ string, paths *[]string, _ string) {
			(*paths)[0], (*paths)[1] = (*paths)[1], (*paths)[0]
		}, 1, false},
		{"file replaced by a directory", func(t *testing.T, dir string, _ *[]string, _ string) {
			p := filepath.Join(dir, "b")
			if err := os.Remove(p); err != nil || os.Mkdir(p, 0o755) != nil {
				t.Fatal(err)
			}
		}, 1, false},
		{"empty cache file", func(t *testing.T, _ string, _ *[]string, cache string) {
			if err := os.WriteFile(cache, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, 1, false},
		{"truncated cache file", func(t *testing.T, _ string, _ *[]string, cache string) {
			data, err := os.ReadFile(cache)
			if err != nil || os.WriteFile(cache, data[:len(data)-1], 0o600) != nil {
				t.Fatal(err)
			}
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
			writeFile(t, paths[1], "bbb", base) // a and b stamps differ: their order counts
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
		if want := (Stamp{Size: int64(i), MtimeNs: int64(1000+i) * 1e9}); seq[i].Size != want.Size || seq[i].MtimeNs != want.MtimeNs {
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
	if !ok || s.Size != 5 || s.MtimeNs != 5000e9 {
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

func TestWriteGobReplacesAndReadsBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "dir")
	path := filepath.Join(dir, "x.gob")
	type rec struct {
		Name string
		N    []int
	}
	if !WriteGob(path, rec{"a", []int{1}}) {
		t.Fatal("WriteGob failed")
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(dir); err != nil || st.Mode().Perm() != 0o700 {
			t.Fatalf("created directory mode %v, %v; want 0700", st.Mode(), err)
		}
	}
	// A 0644 file already there is replaced by a 0600 one with the new value.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if !WriteGob(path, &rec{"b", []int{2, 3}}) {
		t.Fatal("WriteGob over an existing file failed")
	}
	var got rec
	if !ReadGob(path, &got) || got.Name != "b" || len(got.N) != 2 || got.N[1] != 3 {
		t.Fatalf("ReadGob = %+v", got)
	}
	if st, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || st.Mode().Perm() != 0o600) {
		t.Fatalf("replaced file mode %v, %v; want 0600", st.Mode(), err)
	}
	// Under a regular file nothing can be written, and nothing is left beside it.
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if WriteGob(filepath.Join(blocker, "y.gob"), rec{}) {
		t.Fatal("WriteGob under a regular file succeeded")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 2 {
		t.Fatalf("dir holds %d entries, want x.gob and file", len(ents))
	}
}

func TestReadGobRejects(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.gob")
	if !WriteGob(good, map[string]int{"a": 1}) {
		t.Fatal("WriteGob failed")
	}
	data, _ := os.ReadFile(good)
	for name, body := range map[string][]byte{"empty": {}, "garbage": []byte("garbage"), "truncated": data[:len(data)-1]} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
		var m map[string]int
		if ReadGob(p, &m) {
			t.Errorf("%s decoded as %v", name, m)
		}
	}
	var m map[string]int
	if ReadGob(filepath.Join(dir, "missing"), &m) || ReadGob(dir, &m) {
		t.Error("a missing file or a directory decoded")
	}
	var wrong struct{ B []string }
	if ReadGob(good, &wrong) {
		t.Error("a map decoded into an unrelated struct")
	}
	if !ReadGob(good, &m) || m["a"] != 1 {
		t.Errorf("ReadGob = %v", m)
	}
}

func TestFilesWorkerCounts(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := range 7 {
		p := filepath.Join(dir, string(rune('a'+i)))
		writeFile(t, p, string(make([]byte, i)), time.Unix(int64(2000+i), 0))
		paths = append(paths, p)
	}
	want := make([]Stamp, len(paths))
	for i, p := range paths {
		want[i] = Stat(p)
	}
	for _, workers := range []int{-1, 0, 1, 2, 3, 6, 7, 8, 100} {
		for n := 0; n <= len(paths); n++ {
			got := Files(paths[:n], workers)
			if len(got) != n {
				t.Fatalf("Files(%d paths, %d) has %d stamps", n, workers, len(got))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("Files(%d paths, %d)[%d] = %v, want %v", n, workers, i, got[i], want[i])
				}
			}
		}
	}
}

func TestStatFollowsLinksAndMarksMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	writeFile(t, p, "abc", time.Unix(3000, 7))
	if s := Stat(p); s.Size != 3 || s.MtimeNs != 3000e9+7 || !s.Exists() {
		t.Fatalf("Stat = %v", s)
	}
	st, _ := os.Stat(p)
	if Of(st) != Stat(p) {
		t.Fatal("Of and Stat differ")
	}
	link, broken := filepath.Join(dir, "link"), filepath.Join(dir, "broken")
	if os.Symlink(p, link) != nil || os.Symlink(filepath.Join(dir, "nowhere"), broken) != nil {
		t.Skip("no symlinks")
	}
	if Stat(link) != Stat(p) {
		t.Fatal("Stat of a link is not its target's")
	}
	if Stat(broken) != Missing || Same(broken, Missing) {
		t.Fatal("a broken link is not Missing, or Missing is Same")
	}
	// Missing is never Same, even for a path that is still missing.
	if Same(filepath.Join(dir, "gone"), Missing) {
		t.Fatal("Same(missing, Missing)")
	}
}

func TestSwitchAndNamesEdges(t *testing.T) {
	t.Setenv("STATCACHE_TEST_SWITCH", "\tNo\n")
	if !Off("STATCACHE_TEST_SWITCH") {
		t.Error("tab/newline-wrapped No does not turn the cache off")
	}
	if Off("STATCACHE_TEST_UNSET_SWITCH") {
		t.Error("an unset variable turns the cache off")
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if runtime.GOOS != "windows" && runtime.GOOS != "plan9" {
		if got := UserDir("niagara-go", "x"); got != "" {
			t.Errorf("UserDir without a home = %q, want \"\"", got)
		}
	}
	for exe, want := range map[string]bool{
		"/b/mod.test": true, "mod.test.exe": true, "/b/.test": true,
		"/b/mod": false, "/b/mod.tests": false, "/b/test": false, "/b/mod.test.bin": false,
	} {
		if IsTestName(exe) != want {
			t.Errorf("IsTestName(%q) = %v, want %v", exe, !want, want)
		}
	}
}

// TestStampSeesAnEditThatKeepsSizeAndMtime: cp -p, rsync -a, tar x and touch -r
// leave size and mtime as they were; the ctime (and, for a replaced file, the
// inode) still tell the stamps apart where the system reports them.
func TestStampSeesAnEditThatKeepsSizeAndMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime and inode are read on Linux only (Sys)")
	}
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := Stat(p)
	stattest.EditKeepingStamp(t, p, []byte("two"))
	if after := Stat(p); after == before || Same(p, before) {
		t.Fatalf("stamp %+v unchanged by an edit keeping size and mtime", after)
	}
	if !Of(mustStat(t, p)).Exists() || Missing.Exists() {
		t.Fatal("Exists")
	}
}

func mustStat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st
}
