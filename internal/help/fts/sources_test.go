package fts

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

const testdata = "../testdata"

func dataRoot(checkout, vid string) string {
	return filepath.Join(checkout, ftsfixture.Versions[vid])
}

func TestSourceDirsBorrowManuals(t *testing.T) {
	checkout := ftsfixture.Checkout(t, testdata, true)
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
	checkout := ftsfixture.Checkout(t, testdata, true)
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
