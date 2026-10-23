package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFreshnessReportsRetiredIndexes: an index file the Go port retired (call-index.json,
// decision I3-5) that is still on disk is named, so it can be deleted by hand; freshness
// never deletes it (it only reads the data root) and the result does not depend on it.
func TestFreshnessReportsRetiredIndexes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	for name, body := range map[string]string{
		".manifest.json":  `{"stamped_utc": "2200-01-01T00:00:00+00:00", "sources": {}}`,
		"call-index.json": "{}",
	} {
		if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "indexes", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: root, Out: &out}).freshness(); err != nil {
		t.Fatal(err)
	}
	want := "Retired index files still on disk (nh neither reads nor rebuilds them; delete by hand):\n" +
		"  indexes/call-index.json\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("freshness:\n%s\nwant:\n%s", out.String(), want)
	}
	if _, err := os.Stat(filepath.Join(root, "indexes", "call-index.json")); err != nil {
		t.Errorf("freshness removed the retired index: %v", err)
	}
	status, err := indexStatus(root)
	if err != nil || !strings.HasSuffix(status, "; retired index left: call-index.json") {
		t.Errorf("--all status %q, %v", status, err)
	}
}
