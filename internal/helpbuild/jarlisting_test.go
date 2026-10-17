package helpbuild

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// test_install_sweep.HELP_JAR.
var helpJar = [][2]string{
	{"doc/About-Help.html", "<html><p>helpzork page</p></html>"},
	{"com/acme/BHelp.class", "\xca\xfe"},
}

func TestCachePathHashesTheAbsoluteInstallPath(t *testing.T) {
	install := t.TempDir()
	sum := sha1.Sum([]byte(install))
	want := filepath.Join("/repo", ".cache", "jar-listings", hex.EncodeToString(sum[:])[:16]+".json")
	got, err := CachePath("/repo", install)
	if err != nil || got != want {
		t.Fatalf("CachePath = %q, %v; want %q", got, err, want)
	}
}

// test_class_entries_are_counted_not_listed.
func TestClassEntriesAreCountedNotListed(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "helpMod-rt.jar")
	writeJar(t, jar, helpJar...)
	l := ReadListing(jar)
	if l == nil {
		t.Fatal("listing is nil")
	}
	if l.Classes != 1 || len(l.Entries) != 1 || l.Entries[0].Name != "doc/About-Help.html" {
		t.Fatalf("listing %+v", l)
	}
	if l.Entries[0].Size != uint64(len(helpJar[0][1])) || l.Entries[0].CRC != crc32.ChecksumIEEE([]byte(helpJar[0][1])) {
		t.Fatalf("size/crc %+v", l.Entries[0])
	}
}

// Directories are dropped, duplicates and empty entries kept in central
// directory order, ".CLASS" is not a class (Python's endswith is exact).
func TestListingKeepsCentralDirectoryOrder(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "a.jar")
	writeJar(t, jar, [2]string{"doc/", ""}, [2]string{"doc/b.html", "b"},
		[2]string{"doc/a.html", "a"}, [2]string{"doc/b.html", "bb"},
		[2]string{"doc/empty.txt", ""}, [2]string{"X.CLASS", "x"})
	l := ReadListing(jar)
	var names []string
	for _, e := range l.Entries {
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "doc/b.html,doc/a.html,doc/b.html,doc/empty.txt,X.CLASS" || l.Classes != 0 {
		t.Fatalf("names %q classes %d", names, l.Classes)
	}
}

// listingFixture is three jars of test_install_sweep.make_install.
func listingFixture(t *testing.T) (dir string, jars []string) {
	t.Helper()
	dir = t.TempDir()
	jars = []string{filepath.Join(dir, "modules", "helpMod-rt.jar"),
		filepath.Join(dir, "modules", "lib-rt.jar"), filepath.Join(dir, "modules", "thing-ux.jar")}
	writeJar(t, jars[0], helpJar...)
	writeJar(t, jars[1], [2]string{"org/apache/words.txt", "a\nb\n"}, [2]string{"com/acme/BLib.class", "\xca\xfe"})
	writeJar(t, jars[2], [2]string{"rc/notes.md", "# Notes\n"}, [2]string{"rc/thing.js", "var x;"})
	return dir, jars
}

// test_second_sweep_reads_no_jar_and_changed_jar_is_relisted.
func TestSecondReadOpensNoJarAndChangedJarIsRelisted(t *testing.T) {
	dir, jars := listingFixture(t)
	path := filepath.Join(dir, "cache", "listings.json")
	first := NewListings(path)
	for _, j := range jars {
		first.Listing(j)
	}
	if first.Reads != 3 {
		t.Fatalf("first reads %d", first.Reads)
	}
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}
	second := NewListings(path)
	for _, j := range jars {
		if second.Listing(j) == nil {
			t.Fatalf("%s: nil from the cache", j)
		}
	}
	if second.Reads != 0 {
		t.Fatalf("second reads %d, want 0", second.Reads)
	}
	writeJar(t, jars[1], [2]string{"org/apache/words.txt", "a\nb\nc\n"})
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(jars[1], later, later); err != nil {
		t.Fatal(err)
	}
	third := NewListings(path)
	for _, j := range jars {
		third.Listing(j)
	}
	if third.Reads != 1 {
		t.Fatalf("third reads %d, want 1", third.Reads)
	}
	names, ok := third.Names(jars[1])
	if !ok || strings.Join(names, ",") != "org/apache/words.txt" {
		t.Fatalf("names %q %v", names, ok)
	}
}

// test_damaged_cache_is_rebuilt.
func TestDamagedCacheIsRebuilt(t *testing.T) {
	dir, jars := listingFixture(t)
	for _, bad := range []string{"{not json", "[1, 2]", `{"version": 2, "jars": {"x": {}}}`, "\xef\xbb\xbf{}"} {
		path := filepath.Join(dir, "bad.json")
		writeFile(t, path, []byte(bad))
		ls := NewListings(path)
		if ls.Len() != 0 {
			t.Fatalf("%q: %d cached jars, want 0", bad, ls.Len())
		}
		if ls.Listing(jars[0]) == nil {
			t.Fatalf("%q: no listing", bad)
		}
	}
}

// The cache file is Python's compact json.dumps, byte for byte.
func TestCacheFileBytes(t *testing.T) {
	dir, jars := listingFixture(t)
	path := filepath.Join(dir, "c.json")
	ls := NewListings(path)
	ls.Listing(jars[0])
	ls.Listing(filepath.Join(dir, "modules", "missing.jar")) // stat fails: not cached
	if err := ls.Save(); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(jars[0])
	want := fmt.Sprintf(`{"version":1,"jars":{%q:{"stamp":[%d,%d],"listing":{"entries":[["doc/About-Help.html",%d,%d]],"classes":1}}}}`,
		jars[0], st.Size(), st.ModTime().UnixNano(), len(helpJar[0][1]), crc32.ChecksumIEEE([]byte(helpJar[0][1])))
	if got := readText(t, path); got != want {
		t.Fatalf("cache\n got %s\nwant %s", got, want)
	}
}

// An unreadable jar is cached as null and not reopened while unchanged.
func TestUnreadableJarIsCachedAsNull(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "corrupt.jar")
	writeFile(t, jar, []byte("not a zip"))
	path := filepath.Join(dir, "c.json")
	ls := NewListings(path)
	if ls.Listing(jar) != nil || ls.Reads != 1 {
		t.Fatalf("reads %d", ls.Reads)
	}
	if err := ls.Save(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readText(t, path), `"listing":null`) {
		t.Fatalf("cache %s", readText(t, path))
	}
	again := NewListings(path)
	if again.Listing(jar) != nil || again.Reads != 0 {
		t.Fatalf("reads %d, want 0", again.Reads)
	}
	if _, ok := again.Names(jar); ok {
		t.Fatal("Names of an unreadable jar reported ok")
	}
}

func TestSaveWritesOnlyWhenSomethingWasListed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	if err := NewListings(path).Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cache written without a listing: %v", err)
	}
	mem := NewListings("")
	_, jars := listingFixture(t)
	mem.Listing(jars[0])
	if err := mem.Save(); err != nil {
		t.Fatal(err)
	}
}
