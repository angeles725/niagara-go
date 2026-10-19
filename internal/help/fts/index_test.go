package fts

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture/rawfixture"
)

// TestFingerprintMatchesPython compares Fingerprint with the one Python
// stored in each fixture index (fulltext_gen.py), including 2.0's docs-text
// borrowed from 1.0 and the devguide-examples files that are skipped. Those
// were computed on Linux from the checkout's paths and mtimes, so they hold
// only there; the Windows Python that recomputed them on the Windows host
// retired with D47.
func TestFingerprintMatchesPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stored fingerprints are a Linux run's: Windows paths and mtime resolution differ")
	}
	checkout := rawfixture.Checkout(t, testdata, false)
	for vid := range rawfixture.Versions {
		base := dataRoot(checkout, vid)
		got, want := Fingerprint(SourceDirs(base)), storedFingerprint(DBPath(base))
		if want == "" || got != want {
			t.Errorf("%s fingerprint:\n got %s\nwant %s", vid, got, want)
		}
	}
}

func TestOpenRefusesMissingAndStaleIndexes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := rawfixture.Checkout(t, testdata, true)
	var stale *StaleIndexError
	if _, err := Open(checkout, "1.0"); !errors.As(err, &stale) || !stale.Missing {
		t.Fatalf("missing index: %v", err)
	}
	want := "full-text index indexes/fulltext.sqlite under " + checkout + " is missing; this reader " +
		"never builds it, rebuild it with: nh --niagara-version 1.0 find index"
	if stale.Error() != want {
		t.Errorf("error:\n got %s\nwant %s", stale.Error(), want)
	}

	rawfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), DBPath(checkout), checkout)
	rawfixture.Refresh(t, DBPath(checkout), checkout, hostFingerprint)
	x, err := Open(checkout, "1.0")
	if err != nil {
		t.Fatalf("fresh index: %v", err)
	}
	x.Close()

	// Any change to an indexed file's mtime makes the stored fingerprint stale
	// (one NTFS tick, 100 ns, so Windows records the change too).
	touched := filepath.Join(checkout, "guides-clean", "index.txt")
	later := time.Unix(0, rawfixture.FixedNS+100)
	if err := os.Chtimes(touched, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(checkout, ""); !errors.As(err, &stale) || stale.Missing ||
		!strings.Contains(err.Error(), "is out of date") || strings.Contains(err.Error(), "--niagara-version") {
		t.Errorf("stale index: %v", err)
	}
}

func TestOpenFallsBackToTheCacheCopy(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	checkout := rawfixture.Checkout(t, testdata, true)
	path := CachePath(checkout)
	if !strings.HasPrefix(path, filepath.Join(cache, "niagara-help")+string(filepath.Separator)) || len(filepath.Base(filepath.Dir(path))) != 16 {
		t.Fatalf("CachePath = %s", path)
	}
	// A stale indexes/ copy is passed over for an up-to-date cache copy.
	rawfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), DBPath(checkout), checkout)
	rawfixture.SetFingerprint(t, DBPath(checkout), "engine=2", "", "")
	rawfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), path, checkout)
	rawfixture.Refresh(t, path, checkout, hostFingerprint)
	x, err := Open(checkout, "1.0")
	if err != nil {
		t.Fatalf("cache copy: %v", err)
	}
	x.Close()
}

func TestStoredFingerprintOfAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fulltext.sqlite")
	if err := os.WriteFile(path, []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fp := storedFingerprint(path); fp != "" {
		t.Errorf("storedFingerprint = %q, want empty", fp)
	}
}

func TestMatchExpr(t *testing.T) {
	groups := []Group{{"alarm", "registr*"}, {"lead lag"}}
	if got, want := matchExpr(groups, "and"), `("alarm" OR "registr"*) AND ("lead lag")`; got != want {
		t.Errorf("and: %s", got)
	}
	if got, want := matchExpr(groups, "or"), `("alarm" OR "registr"*) OR ("lead lag")`; got != want {
		t.Errorf("or: %s", got)
	}
}
