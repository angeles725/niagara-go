package callgraph

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
)

// fixtureIndex is the callers/callees golden fixture's call graph index.
func fixtureIndex(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixture", "cg", "indexes", "callgraph-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newCachedBase writes the fixture index under a temporary base directory and
// lowers the cache floor so that it is cached; it returns the index path.
func newCachedBase(t *testing.T) (base, indexPath string) {
	t.Helper()
	old := cacheMinBytes
	cacheMinBytes = 0
	t.Cleanup(func() { cacheMinBytes = old })
	base = t.TempDir()
	indexPath = filepath.Join(base, "indexes", "callgraph-index.json")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, fixtureIndex(t), 0o644); err != nil {
		t.Fatal(err)
	}
	return base, indexPath
}

// decoded is the Graph of the index at path, decoded from its JSON.
func decoded(t *testing.T, path string) *Graph {
	t.Helper()
	v, err := index.LoadPython(path)
	if err != nil {
		t.Fatal(err)
	}
	return New(v.(*index.Object))
}

// sameGraph compares two graphs by their edges and their scope line (_meta).
func sameGraph(t *testing.T, got, want *Graph) {
	t.Helper()
	if got == nil {
		t.Fatal("no graph")
	}
	if !reflect.DeepEqual(got.callers, want.callers) || !reflect.DeepEqual(got.callees, want.callees) ||
		got.Scope() != want.Scope() {
		t.Errorf("cached graph differs:\n%q %q %q\nwant\n%q %q %q",
			got.callers, got.callees, got.Scope(), want.callers, want.callees, want.Scope())
	}
}

func TestCacheRoundTrip(t *testing.T) {
	base, path := newCachedBase(t)
	want := decoded(t, path)
	if g := NewCache(path, CachePath(base)).Load(); g != nil {
		t.Fatal("hit before any store")
	}
	if !NewCache(path, CachePath(base)).Store(want) {
		t.Fatal("Store failed")
	}
	st, err := os.Stat(CachePath(base))
	if err != nil || runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 { // NTFS keeps no POSIX mode
		t.Fatalf("cache file: %v, %v (want mode 0600)", st, err)
	}
	sameGraph(t, NewCache(path, CachePath(base)).Load(), want)
	if got := NewCache(path, CachePath(base)).Load().CallersOfClass(nil, "BTarget"); len(got.Methods) == 0 {
		t.Error("the cached graph answers no lookup")
	}
}

// TestCacheSharedThroughSymlink: a base directory reached through a link hits
// the cache written through the real path.
func TestCacheSharedThroughSymlink(t *testing.T) {
	base, path := newCachedBase(t)
	NewCache(path, CachePath(base)).Store(decoded(t, path))
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(base, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("no symlink privilege: %v", err)
		}
		t.Fatal(err)
	}
	linked := filepath.Join(link, "indexes", "callgraph-index.json")
	sameGraph(t, NewCache(linked, CachePath(link)).Load(), decoded(t, path))
}

func TestCacheIgnoredWhenIndexChanged(t *testing.T) {
	base, path := newCachedBase(t)
	NewCache(path, CachePath(base)).Store(decoded(t, path))
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if NewCache(path, CachePath(base)).Load() != nil {
		t.Error("a cache of an index with another mtime was used")
	}
}

func TestCacheIgnoredWhenCorrupt(t *testing.T) {
	base, path := newCachedBase(t)
	if err := os.MkdirAll(filepath.Dir(CachePath(base)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CachePath(base), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if NewCache(path, CachePath(base)).Load() != nil {
		t.Error("a corrupt cache was used")
	}
}

// TestFromCacheRejectsInconsistentOffsets: offsets out of order, past the
// names, or leaving names unused make the cache unusable, never a panic.
func TestFromCacheRejectsInconsistentOffsets(t *testing.T) {
	for name, cf := range map[string]cacheFile{
		"past the names":    {Names: "A.aB.b", Ends: []int{3, 9}, Callees: []int{1}},
		"backwards":         {Names: "A.aB.b", Ends: []int{3, 1}, Callees: []int{1}},
		"names left over":   {Names: "A.aB.bC", Ends: []int{3, 6}, Callees: []int{1}},
		"too few offsets":   {Names: "A.aB.b", Ends: []int{3}, Callees: []int{1}},
		"negative count":    {Names: "A.a", Ends: []int{3}, Callees: []int{-1}},
		"offsets left over": {Names: "A.aB.b", Ends: []int{3, 6}, Callees: []int{0}},
	} {
		if g := fromCache(&cf); g != nil {
			t.Errorf("%s: fromCache = %v, want nil", name, g)
		}
	}
	g := fromCache(&cacheFile{Names: "A.aB.b", Ends: []int{3, 6}, Callees: []int{1}})
	if g == nil || !reflect.DeepEqual(g.callers, []string{"A.a"}) || !reflect.DeepEqual(g.callees, [][]string{{"B.b"}}) {
		t.Errorf("consistent file: %+v", g)
	}
}

// TestCacheNotStoredWhenIndexChangedWhileRead: a graph decoded from an index
// that changed after NewCache is not stored.
func TestCacheNotStoredWhenIndexChangedWhileRead(t *testing.T) {
	base, path := newCachedBase(t)
	c := NewCache(path, CachePath(base))
	g := decoded(t, path)
	if err := os.WriteFile(path, append(fixtureIndex(t), ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	if c.Store(g) {
		t.Error("stored a graph of an index that changed while it was read")
	}
	if _, err := os.Stat(CachePath(base)); err == nil {
		t.Error("cache file written")
	}
}

func TestCacheSkipsSmallIndexes(t *testing.T) {
	base, path := newCachedBase(t)
	cacheMinBytes = 1 << 20
	if NewCache(path, CachePath(base)).Store(decoded(t, path)) {
		t.Error("cached an index under the floor")
	}
	if NewCache(filepath.Join(base, "missing.json"), CachePath(base)).Store(decoded(t, path)) {
		t.Error("cached a missing index")
	}
}

// TestCacheUnwritableIsSkipped: a regular file where the cache directory
// belongs makes the cache unwritable (also as root); Store reports it and
// leaves nothing behind.
func TestCacheUnwritableIsSkipped(t *testing.T) {
	base, path := newCachedBase(t)
	blocker := filepath.Dir(CachePath(base))
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if NewCache(path, CachePath(base)).Store(decoded(t, path)) {
		t.Error("Store reported success with no cache directory")
	}
	entries, _ := os.ReadDir(filepath.Join(base, "indexes"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != ".mnav-cache,callgraph-index.json" {
		t.Errorf("indexes/ holds %v, want only the index and the blocking file", names)
	}
}
