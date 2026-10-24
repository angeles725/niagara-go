package fts

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture/rawfixture"
)

const testdata = "../testdata"

func dataRoot(checkout, vid string) string {
	return filepath.Join(checkout, rawfixture.Versions[vid])
}

// hostFingerprint is what rawfixture.Refresh stores on Windows.
func hostFingerprint(base string) string { return Fingerprint(SourceDirs(base)) }

// indexedCheckout is rawfixture.Checkout with its indexes, their fingerprints
// rewritten for the host on Windows (rawfixture.Refresh).
func indexedCheckout(t *testing.T) string {
	t.Helper()
	checkout := rawfixture.Checkout(t, testdata, false)
	rawfixture.RefreshAll(t, checkout, hostFingerprint)
	return checkout
}

func TestSourceDirsBorrowManuals(t *testing.T) {
	checkout := rawfixture.Checkout(t, testdata, true)
	got := SourceDirs(dataRoot(checkout, "2.0"))
	want := []SourceDir{{"guides-clean", filepath.Join(checkout, "versions/2.0/guides-clean")},
		{"docs-text", filepath.Join(checkout, "docs-text")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SourceDirs(2.0) = %v, want %v", got, want)
	}
	if root, owner := DocsTextRoot(dataRoot(checkout, "2.0")); root != checkout || owner != "1.0" {
		t.Errorf("DocsTextRoot(2.0) = %q, %q", root, owner)
	}
	if _, owner := DocsTextRoot(checkout); owner != "" {
		t.Errorf("1.0 owns its manuals, got owner %q", owner)
	}
	// An unregistered data root never borrows.
	if root, owner := SharedRoot(filepath.Join(checkout, "versions"), "docs-text"); owner != "" ||
		root != filepath.Join(checkout, "versions") {
		t.Errorf("unregistered root borrowed: %q %q", root, owner)
	}
}

func TestIterFilesSkipsBundledAndMinified(t *testing.T) {
	checkout := rawfixture.Checkout(t, testdata, true)
	dir := filepath.Join(checkout, "devguide-examples")
	var got []string
	for _, p := range IterFiles("devguide-examples", dir) {
		got = append(got, rel(dir, p))
	}
	if want := []string{"web/app.js", "web/page.html"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IterFiles = %q, want %q", got, want)
	}
	if n := CountFiles(dir); n != 4 {
		t.Errorf("CountFiles = %d, want every file (4)", n)
	}
}

// A module example is a template of a module jar (IsModuleExample); a
// vendored copy (node_modules, rc/vendor, rc/ext, a top-level ext/) is not,
// and module-examples/ never lists one: its first folder is the jar stem.
func TestModuleExamples(t *testing.T) {
	for entry, want := range map[string]bool{
		"rc/examples/px/LineView.px": true, "rc/fe/templates/a.hbs": true, "com/x/rc/page.vm": true,
		"bog/Niagara.bog": true, "rc/app.js": false, "rc/A.PX": false, "rc/": false,
		"rc/vendor/lib/a.hbs": false, "rc/ext/a.hbs": false, "ext/a.hbs": false, "rc/x/node_modules/y/a.hbs": false,
	} {
		if got := IsModuleExample(entry); got != want {
			t.Errorf("IsModuleExample(%q) = %v, want %v", entry, got, want)
		}
	}
	dir := t.TempDir()
	for _, rel := range []string{"kit-rt/rc/a.px", "kit-rt/rc/vendor/b.hbs", "ext/c.vm", "kit-rt/notes.txt"} {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, p := range IterFiles("module-examples", dir) {
		got = append(got, rel(dir, p))
	}
	if want := []string{"ext/c.vm", "kit-rt/rc/a.px"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IterFiles = %q, want %q", got, want)
	}
}
