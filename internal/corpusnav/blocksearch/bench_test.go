package blocksearch

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkFindWarm is one warm corpus-nav find (fresh cache) on the real corpus named by
// $CORPUS_NAV_ROOT (a niagara-research checkout); skipped without one. The cache lives in
// a temporary directory, built by the first, untimed Find.
func BenchmarkFindWarm(b *testing.B) {
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		b.Skip("set CORPUS_NAV_ROOT to a niagara-research checkout")
	}
	if _, err := os.Stat(filepath.Join(root, "CATALOG.md")); err != nil {
		b.Skipf("no corpus at %s: %v", root, err)
	}
	b.Setenv("CORPUS_NAV_FIND_INDEX", "")
	for _, q := range []Query{
		{Text: "BACnet COV subscription", Scope: "blocks", Limit: 50},
		{Text: "alarm escalation", Scope: "blocks", Limit: 50},
		{Text: "fox tunnel certificate", Scope: "blocks", Limit: 50, RequireAll: true},
	} {
		b.Run(q.Text, func(b *testing.B) {
			c := New(root, b.TempDir())
			if r := c.Find(q); r.CacheNote == "" {
				b.Fatalf("first find did not build the cache")
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if r := c.Find(q); r.CacheNote != "" {
					b.Fatalf("warm find fell back: %s", r.CacheNote)
				}
			}
		})
	}
}
