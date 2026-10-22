package nav

import (
	"bytes"
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/navbuild/reindex"
)

// The tests route reindex as mnav does: with the rebuild plugged in, as
// cmd/mnav's main plugs it.
func init() { UseReindex(reindex.Main) }

// TestReindexWithoutBuilder: a binary whose main did not plug the rebuild in
// reports the wiring defect (exit 2), never a silent success.
func TestReindexWithoutBuilder(t *testing.T) {
	saved := reindexMain
	t.Cleanup(func() { UseReindex(saved) })
	UseReindex(nil)
	var out, errb bytes.Buffer
	code := Main([]string{"reindex", "--check"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != "mnav: the index builder is not linked into this binary\n" {
		t.Errorf("reindex without builder: exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

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

// TestReaderDoesNotLinkTheBuilders pins the package rule of reindex.go: the
// reader (internal/nav and every package under it) never depends on the
// index builders (internal/navbuild), which reach it only through UseReindex
// in cmd/mnav; and the builders never depend on the reader's root package.
func TestReaderDoesNotLinkTheBuilders(t *testing.T) {
	const builders = modulePath + "/internal/navbuild"
	var readers []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if m, _ := filepath.Glob(filepath.Join(path, "*.go")); len(m) > 0 {
			readers = append(readers, modulePath+"/internal/nav"+strings.TrimPrefix("/"+filepath.ToSlash(path), "/."))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(readers, modulePath+"/internal/nav/core") {
		t.Fatalf("reader packages %q miss internal/nav/core: the walk is broken", readers)
	}
	for _, goos := range []string{"linux", "windows"} {
		for dep := range moduleDeps(t, goos, readers...) {
			if dep == builders || strings.HasPrefix(dep, builders+"/") {
				t.Errorf("%s: the reader depends on the builder package %s; plug it in through UseReindex in cmd/mnav instead",
					goos, strings.TrimPrefix(dep, modulePath+"/"))
			}
		}
		if moduleDeps(t, goos, builders+"/reindex")[modulePath+"/internal/nav"] {
			t.Errorf("%s: internal/navbuild/reindex depends on internal/nav", goos)
		}
	}
}
