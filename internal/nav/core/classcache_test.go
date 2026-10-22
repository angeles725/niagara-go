package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeClassIndex writes indexes/class-index.json under base: n classes,
// each record padded with pad bytes, plus a _meta and every JSON value kind.
func writeClassIndex(t *testing.T, base string, n, pad int, tag string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"_meta": {"description": "fixture ` + tag + `", "total_entries": ` + fmt.Sprint(n) +
		`, "ratio": 1.5e3, "neg": -0.25, "none": null, "flag": true, "off": false, "empty": {}, "list": []}, "classes": {`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `"B%d": [{"package": "p.q", "module": "m-rt", "kind": "class", "extends": "BComponent", `+
			`"implements": ["I%d", "café"], "lines": %d, "zkm": false, "outer_class": null, "pad": "%s"}]`,
			i, i%7, i, strings.Repeat("x", pad))
	}
	b.WriteString(`}, "dup": 1, "dup": 2}`)
	p := IndexPath(base, "class-index.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// loadClassIndex runs one command's LoadClassIndex: the dumped index and stderr.
func loadClassIndex(t *testing.T, base string) (string, string) {
	t.Helper()
	var errw bytes.Buffer
	r := NewAt(base, base, &bytes.Buffer{}, &errw)
	o := r.LoadClassIndex()
	if o == nil {
		t.Fatalf("class index did not load: %s", errw.String())
	}
	return Dumps(o, -1), errw.String()
}

func useSmallClassCache(t *testing.T) {
	old := classCacheMinBytes
	classCacheMinBytes = 1
	t.Cleanup(func() { classCacheMinBytes = old })
}

// TestClassIndexCacheHitMatchesMiss: the second load is a cache hit (the
// JSON is unreadable, yet it loads) with the same index and the same stderr,
// progress line included.
func TestClassIndexCacheHitMatchesMiss(t *testing.T) {
	useSmallClassCache(t)
	base := t.TempDir()
	p := writeClassIndex(t, base, 22000, 900, "a") // over the 20 MB progress line
	miss, missErr := loadClassIndex(t, base)
	if !strings.Contains(missErr, "  Loading class-index.json (") || !strings.HasSuffix(missErr, "... OK\n") {
		t.Fatalf("miss stderr has no progress line: %q", missErr)
	}
	cache := ClassIndexCachePath(base)
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("no cache written: %v", err)
	}
	// Corrupt the JSON in place, same size and mtime: only the cache can answer.
	st, _ := os.Stat(p)
	data, _ := os.ReadFile(p)
	data[len(data)/2] = '{'
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	hit, hitErr := loadClassIndex(t, base)
	if hit != miss {
		t.Error("cache hit decodes a different index than the JSON")
	}
	if hitErr != missErr {
		t.Errorf("stderr differs: hit %q, miss %q", hitErr, missErr)
	}
}

// TestClassIndexCacheInvalidation: a changed index (size or mtime), or a
// malformed cache, is decoded from the JSON and the cache rewritten.
func TestClassIndexCacheInvalidation(t *testing.T) {
	useSmallClassCache(t)
	base := t.TempDir()
	p := writeClassIndex(t, base, 50, 3, "a")
	first, _ := loadClassIndex(t, base)

	writeClassIndex(t, base, 50, 3, "b") // same size, new content
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	second, _ := loadClassIndex(t, base)
	if second == first || !strings.Contains(second, "fixture b") {
		t.Fatal("an index with a new mtime was served from the stale cache")
	}

	writeClassIndex(t, base, 51, 3, "b") // new size
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	third, _ := loadClassIndex(t, base)
	if !strings.Contains(third, `"B50"`) {
		t.Fatal("an index with a new size was served from the stale cache")
	}

	if err := os.WriteFile(ClassIndexCachePath(base), []byte("not a gob"), 0o600); err != nil {
		t.Fatal(err)
	}
	if again, _ := loadClassIndex(t, base); again != third {
		t.Fatal("a malformed cache changed the loaded index")
	}
	if again, _ := loadClassIndex(t, base); again != third {
		t.Fatal("the rewritten cache changed the loaded index")
	}
}

// TestClassIndexCacheSkipsSmallIndexes: indexes under the threshold leave
// no cache behind (the read-only fixtures stay untouched).
func TestClassIndexCacheSkipsSmallIndexes(t *testing.T) {
	base := t.TempDir()
	writeClassIndex(t, base, 5, 1, "a")
	loadClassIndex(t, base)
	if _, err := os.Stat(filepath.Dir(ClassIndexCachePath(base))); !os.IsNotExist(err) {
		t.Fatalf("a small index created the cache directory: %v", err)
	}
}
