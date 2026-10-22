package help

import (
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/angeles725/niagara-go"

// moduleDeps is every package of this module that pkgs (module import paths)
// import, directly or not, without their tests, as goos builds them. It reads
// the import lines with go/build (no go command, no network).
func moduleDeps(t *testing.T, goos string, pkgs ...string) map[string]bool {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root not found: %v", err)
	}
	ctx := build.Default
	ctx.GOOS, ctx.CgoEnabled = goos, false
	seen := map[string]bool{}
	var visit func(path string)
	visit = func(path string) {
		p, err := ctx.ImportDir(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path, modulePath))), 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, modulePath+"/") && !seen[imp] {
				seen[imp] = true
				visit(imp)
			}
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
	return seen
}

// TestReaderDoesNotLinkTheBuilders pins the package rule of builders.go: the
// reader (internal/help and every package under it) never depends on the
// index builders (internal/helpbuild), which reach it only through
// buildapi.Builders plugged in by cmd/nh; and the builders never depend on
// the reader's root package. The one exception is internal/helpbuild/fulltext,
// which help/scope runs when a search finds the full-text index missing or
// stale (nsearch and find3 search through help/scope without nh's main).
func TestReaderDoesNotLinkTheBuilders(t *testing.T) {
	const builders = modulePath + "/internal/helpbuild"
	allowed := map[string]bool{builders + "/fulltext": true}
	var readers []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if m, _ := filepath.Glob(filepath.Join(path, "*.go")); len(m) > 0 {
			readers = append(readers, modulePath+"/internal/help"+strings.TrimPrefix("/"+filepath.ToSlash(path), "/."))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(readers, modulePath+"/internal/help/scope") {
		t.Fatalf("reader packages %q miss internal/help/scope: the walk is broken", readers)
	}
	for _, goos := range []string{"linux", "windows"} {
		for dep := range moduleDeps(t, goos, readers...) {
			if (dep == builders || strings.HasPrefix(dep, builders+"/")) && !allowed[dep] {
				t.Errorf("%s: the reader depends on the builder package %s; plug it in through buildapi.Builders in cmd/nh instead",
					goos, strings.TrimPrefix(dep, modulePath+"/"))
			}
		}
		if moduleDeps(t, goos, builders)[modulePath+"/internal/help"] {
			t.Errorf("%s: internal/helpbuild depends on internal/help; it may import only buildapi and the shared leaves", goos)
		}
	}
}
