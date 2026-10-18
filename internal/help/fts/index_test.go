package fts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// TestFingerprintMatchesPython compares Fingerprint with the one Python
// stored in each fixture index (fulltext_gen.py), including 2.0's docs-text
// borrowed from 1.0 and the devguide-examples files that are skipped. Those
// were computed on Linux; on Windows the Windows Python ($PYCOMPAT_WINPY, with
// niagara-help from $NIAGARA_RESEARCH) computes them on the same checkout.
func TestFingerprintMatchesPython(t *testing.T) {
	checkout := ftsfixture.Checkout(t, testdata, false)
	for vid := range ftsfixture.Versions {
		base := dataRoot(checkout, vid)
		got, want := Fingerprint(SourceDirs(base)), storedFingerprint(DBPath(base))
		if runtime.GOOS == "windows" {
			want = winPythonFingerprint(t, base)
		}
		if want == "" || got != want {
			t.Errorf("%s fingerprint:\n got %s\nwant %s", vid, got, want)
		}
	}
}

// winPythonFingerprint is fulltext_index._fingerprint(text_sources.source_dirs(base))
// run by the Windows Python; the test skips when either variable is unset.
func winPythonFingerprint(t *testing.T, base string) string {
	t.Helper()
	py, nr := os.Getenv("PYCOMPAT_WINPY"), os.Getenv("NIAGARA_RESEARCH")
	if py == "" || nr == "" {
		t.Skip("PYCOMPAT_WINPY or NIAGARA_RESEARCH unset: the Linux-built fingerprints never match " +
			"on Windows and no Windows Python computes them (WSLENV=NIAGARA_RESEARCH/p)")
	}
	const script = "import sys; sys.path[:0] = [sys.argv[1]]\n" +
		"from niagara_help_lib import fulltext_index, text_sources\n" +
		"sys.stdout.write(fulltext_index._fingerprint(text_sources.source_dirs(sys.argv[2])))"
	out, err := exec.Command(py, "-I", "-c", script, filepath.Join(nr, "niagara-help", "tools"), base).Output()
	if err != nil {
		t.Fatalf("%s fingerprint: %v", py, err)
	}
	return string(out)
}

func TestOpenRefusesMissingAndStaleIndexes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, true)
	var stale *StaleIndexError
	if _, err := Open(checkout, "1.0"); !errors.As(err, &stale) || !stale.Missing {
		t.Fatalf("missing index: %v", err)
	}
	want := "full-text index indexes/fulltext.sqlite under " + checkout + " is missing; this port " +
		"reads it but never builds it, rebuild it with: python3 tools/niagara_help.py " +
		"--niagara-version 1.0 find index"
	if stale.Error() != want {
		t.Errorf("error:\n got %s\nwant %s", stale.Error(), want)
	}

	ftsfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), DBPath(checkout), checkout)
	ftsfixture.Refresh(t, DBPath(checkout), checkout, hostFingerprint)
	x, err := Open(checkout, "1.0")
	if err != nil {
		t.Fatalf("fresh index: %v", err)
	}
	x.Close()

	// Any change to an indexed file's mtime makes the stored fingerprint stale
	// (one NTFS tick, 100 ns, so Windows records the change too).
	touched := filepath.Join(checkout, "guides-clean", "index.txt")
	later := time.Unix(0, ftsfixture.FixedNS+100)
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
	checkout := ftsfixture.Checkout(t, testdata, true)
	path := CachePath(checkout)
	if !strings.HasPrefix(path, filepath.Join(cache, "niagara-help")+string(filepath.Separator)) || len(filepath.Base(filepath.Dir(path))) != 16 {
		t.Fatalf("CachePath = %s", path)
	}
	// A stale indexes/ copy is passed over for an up-to-date cache copy.
	ftsfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), DBPath(checkout), checkout)
	ftsfixture.SetFingerprint(t, DBPath(checkout), "engine=2", "", "")
	ftsfixture.Install(t, filepath.Join(testdata, "fulltext-db", "1.0.sqlite"), path, checkout)
	ftsfixture.Refresh(t, path, checkout, hostFingerprint)
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
