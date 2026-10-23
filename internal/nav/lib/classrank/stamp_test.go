package classrank

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/statcache/stattest"
)

// TestStatOfSeesAnEditThatKeepsSizeAndMtime: cp -p, rsync -a and touch -r keep
// an index's size and mtime; the ranker cache key still changes.
func TestStatOfSeesAnEditThatKeepsSizeAndMtime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ctime and inode are read on Linux only (statcache.Sys)")
	}
	p := filepath.Join(t.TempDir(), "class-index.json")
	if err := os.WriteFile(p, []byte(`{"a": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := statOf(p)
	stattest.EditKeepingStamp(t, p, []byte(`{"a": 9}`))
	if after := statOf(p); after == before || !after.Present {
		t.Fatalf("stat %+v unchanged by an edit keeping size and mtime", after)
	}
}
