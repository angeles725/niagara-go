package modjar

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim, and every case must decide as it did.

func legacyDefaultDir(testBinary bool) string {
	switch strings.ToLower(pycompat.Strip(os.Getenv(envCache))) {
	case "off", "0", "no", "false":
		return ""
	}
	if testBinary {
		return ""
	}
	home := os.Getenv("XDG_CACHE_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return ""
		}
		home = filepath.Join(h, ".cache")
	}
	return filepath.Join(home, "niagara-go", "modjar")
}

func legacyIsTestName(exe string) bool {
	return strings.HasSuffix(strings.TrimSuffix(filepath.Base(exe), ".exe"), ".test")
}

// legacySame is the stat-again check of Cache.read before statcache.Same.
func legacySame(path string, st os.FileInfo) bool {
	st2, err := os.Stat(path)
	return err == nil && st2.Size() == st.Size() && st2.ModTime().Equal(st.ModTime())
}

func legacyOpenEntries(dir, member string) map[string]cacheEntry {
	sum := sha1.Sum([]byte(member))
	data, err := os.ReadFile(filepath.Join(dir, hex.EncodeToString(sum[:])[:16]+".gob"))
	if err != nil {
		return map[string]cacheEntry{}
	}
	var f cacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&f) == nil && f.Format == cacheFormat &&
		f.Member == member && f.Entries != nil {
		return f.Entries
	}
	return map[string]cacheEntry{}
}

func legacyWriteGob(path string, v any) bool {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(v) != nil {
		return false
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return false
	}
	return atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	}) == nil
}

func TestDefaultDirMatchesLegacy(t *testing.T) {
	for _, sw := range []string{"", "off", " OFF ", "\tno\n", "False", "0", "1", "on", "offline", "of f"} {
		for _, xdg := range []string{"", "/x/cache"} {
			for _, home := range []string{"", "/home/u"} {
				for _, testBinary := range []bool{false, true} {
					t.Setenv(envCache, sw)
					t.Setenv("XDG_CACHE_HOME", xdg)
					t.Setenv("HOME", home)
					if got, want := defaultDir(testBinary), legacyDefaultDir(testBinary); got != want {
						t.Errorf("switch %q xdg %q home %q test %v: %q, legacy %q", sw, xdg, home, testBinary, got, want)
					}
				}
			}
		}
	}
	for _, exe := range []string{"/b/mnav", "/b/modjar.test", `C:\b\modjar.test.exe`, "/b/x.test.exe", "/b/.test", "/b/test", "/b/x.tests"} {
		if got, want := statcache.IsTestName(exe), legacyIsTestName(exe); got != want {
			t.Errorf("IsTestName(%q) = %v, legacy %v", exe, got, want)
		}
	}
}

// TestSameMatchesLegacy: the check that keeps an answer read while the jar
// changed out of the cache decides as before (cache.go: statcache.Same).
func TestSameMatchesLegacy(t *testing.T) {
	base := time.Unix(1_700_000_000, 500)
	cases := []struct {
		name   string
		change func(t *testing.T, p string)
		same   bool
	}{
		{"unchanged", func(*testing.T, string) {}, true},
		{"same size, mtime +1ns", func(t *testing.T, p string) { touch(t, p, base.Add(time.Nanosecond)) }, false},
		{"resized, same mtime", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("longer jar"), 0o644); err != nil {
				t.Fatal(err)
			}
			touch(t, p, base)
		}, false},
		{"rewritten identically", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("jar"), 0o644); err != nil {
				t.Fatal(err)
			}
			touch(t, p, base)
		}, true},
		{"removed", func(t *testing.T, p string) {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.jar")
			if err := os.WriteFile(p, []byte("jar"), 0o644); err != nil {
				t.Fatal(err)
			}
			touch(t, p, base)
			st, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(t, p)
			got, want := statcache.Same(p, statcache.Of(st)), legacySame(p, st)
			if got != want || got != tc.same {
				t.Fatalf("Same = %v, legacy %v; want %v", got, want, tc.same)
			}
		})
	}
}

func touch(t *testing.T, p string, mt time.Time) {
	t.Helper()
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCacheMatchesLegacy(t *testing.T) {
	entries := map[string]cacheEntry{"/j/a.jar": {3, 4, []byte("x"), []string{"META-INF/A.RSA"}, false}}
	files := map[string]any{
		"valid":          cacheFile{cacheFormat, member, entries},
		"other format":   cacheFile{cacheFormat + 1, member, entries},
		"other member":   cacheFile{cacheFormat, "META-INF/other.xml", entries},
		"nil entries":    cacheFile{cacheFormat, member, nil},
		"empty entries":  cacheFile{cacheFormat, member, map[string]cacheEntry{}},
		"another type":   map[string]int{"Format": 1},
		"garbage":        []byte("garbage"),
		"no cache file":  nil,
		"truncated file": "truncate",
	}
	for name, v := range files {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sum := sha1.Sum([]byte(member))
			path := filepath.Join(dir, hex.EncodeToString(sum[:])[:16]+".gob")
			switch v := v.(type) {
			case nil:
			case []byte:
				if err := os.WriteFile(path, v, 0o600); err != nil {
					t.Fatal(err)
				}
			case string:
				legacyWriteGob(path, cacheFile{cacheFormat, member, entries})
				data, _ := os.ReadFile(path)
				if err := os.WriteFile(path, data[:len(data)-2], 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				if !legacyWriteGob(path, v) {
					t.Fatal("legacy write failed")
				}
			}
			got, want := OpenCache(dir, member).entries, legacyOpenEntries(dir, member)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("entries %v, legacy %v", got, want)
			}
			if (name == "valid") != (len(got) == 1) {
				t.Fatalf("entries %v", got)
			}
		})
	}
}

// TestSaveBytesMatchLegacy: Save writes the bytes the legacy writer wrote for
// the same file (one entry: gob writes a map in iteration order).
func TestSaveBytesMatchLegacy(t *testing.T) {
	dir, cacheDir := t.TempDir(), t.TempDir()
	a := filepath.Join(dir, "a.jar")
	writeJar(t, a, entry{member, "A1"}, entry{"META-INF/X.RSA", ""})
	c := OpenCache(cacheDir, member)
	c.ReadAll([]string{a}, 1)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(c.path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(t.TempDir(), "legacy.gob")
	if !legacyWriteGob(legacy, cacheFile{cacheFormat, member, c.entries}) {
		t.Fatal("legacy write failed")
	}
	if want, _ := os.ReadFile(legacy); !bytes.Equal(got, want) {
		t.Fatalf("cache bytes differ from the legacy writer:\n%x\n%x", got, want)
	}
	if st, err := os.Stat(c.path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode %v, %v", st.Mode(), err)
	}
	for _, name := range []string{"cacheFile", "cacheEntry", "NoMember", "MtimeNs"} {
		if !bytes.Contains(got, []byte(name)) {
			t.Errorf("cache bytes do not name %s", name)
		}
	}
}
