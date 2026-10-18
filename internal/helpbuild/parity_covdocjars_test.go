package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
)

// Parity of `coverage --doc-jars`: cmd_doc_jar_coverage through
// testdata/cov/covdriver.py against DocJarCoverage.

// buildingDocs is FulltextDocs as open_index reads it: the up-to-date
// database of root, built first (fulltext.Build) when there is none.
func buildingDocs(root string) (map[string]bool, error) {
	path, _ := fts.FreshPath(root, fts.Fingerprint(fts.SourceDirs(root)))
	if path == "" {
		res, err := fulltext.Build(root, fulltext.Options{})
		if err != nil {
			return nil, err
		}
		path = res.Path
	}
	return FulltextDocsAt(path)
}

// freshDocs is FulltextDocs that never builds: a real corpus must already
// hold an up-to-date index (else Python would rebuild it in place).
func freshDocs(root string) (map[string]bool, error) {
	path, _ := fts.FreshPath(root, fts.Fingerprint(fts.SourceDirs(root)))
	if path == "" {
		return nil, errors.New("the full-text index of " + root + " is missing or stale")
	}
	return FulltextDocsAt(path)
}

// The docjars mode reads the full-text index as open_index does.
func init() {
	covGoModes["docjars"] = func(args ...string) []string { return goCovDocJars(buildingDocs, args...) }
}

// goCovDocJars is the driver's docjars mode: repo, base, optional version.
func goCovDocJars(docs FulltextDocs, args ...string) []string {
	var b strings.Builder
	vid := ""
	if len(args) > 2 {
		vid = args[2]
	}
	st, err := DocJarCoverage(&b, args[0], args[1], vid, docs)
	return covResult(b.String(), intV(st), err)
}

// TestParityDocJarCoverage: the gate on the synthetic install, by version
// id and identified from the data root; with more than FirstGaps gaps; a
// clean install (PASS) and one whose only failure is a guide folder; an
// install that is not there, an unregistered version, a data root without
// indexes. The listing cache each side saves is byte-identical.
func TestParityDocJarCoverage(t *testing.T) {
	covPython(t)
	dir := t.TempDir()
	inst := covInstall(t, dir)
	pyBase, goBase := covCopies(t, covBase(t, dir))
	pyRepo := covRepo(t, filepath.Join(dir, "pyrepo"), "4.99.1.2", inst, pyBase)
	goRepo := covRepo(t, filepath.Join(dir, "gorepo"), "4.99.1.2", inst, goBase)
	both := func(what string, py, gol []string) []string {
		t.Run(what, func(t *testing.T) {
			covBoth(t, what, "docjars", py, gol, pyBase, goBase)
		})
		return nil
	}
	both("by version", []string{pyRepo, pyBase, "4.99.1.2"}, []string{goRepo, goBase, "4.99.1.2"})
	both("unregistered version", []string{pyRepo, pyBase, "4.0.0.0"}, []string{goRepo, goBase, "4.0.0.0"})
	both("no install_dir", []string{pyRepo, pyBase, "9.9.9.9"}, []string{goRepo, goBase, "9.9.9.9"})
	both("no registry", []string{dir, pyBase, "4.99.1.2"}, []string{dir, goBase, "4.99.1.2"})
	pyCache, goCache := fileTexts(t, filepath.Join(pyRepo, ".cache")), fileTexts(t, filepath.Join(goRepo, ".cache"))
	if len(pyCache) != 1 || len(goCache) != 1 {
		t.Fatalf("listing caches: python %d files, go %d", len(pyCache), len(goCache))
	}
	for rel, text := range pyCache {
		if goCache[rel] != text {
			t.Errorf("listing cache %s differs", rel)
		}
	}

	// identified from the data root: a versions/<id>/ root two levels
	// under the repository that registers it
	t.Run("identified", func(t *testing.T) {
		idDir := t.TempDir()
		var roots [2]string
		for i, src := range []string{pyBase, goBase} {
			repo := filepath.Join(idDir, []string{"py", "go"}[i])
			roots[i] = filepath.Join(repo, "versions", "4.99.1.2")
			copyTree(t, src, roots[i])
			writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(
				`{"versions": {"4.99.1.2": {"install_dir": "`+inst+`", "data_root": "versions/4.99.1.2"}}}`))
		}
		covBoth(t, "identified", "docjars",
			[]string{filepath.Dir(filepath.Dir(roots[0])), roots[0]},
			[]string{filepath.Dir(filepath.Dir(roots[1])), roots[1]}, roots[0], roots[1])
		covBoth(t, "not identified", "docjars", []string{dir, pyBase}, []string{dir, goBase}, pyBase, goBase)
	})

	t.Run("many gaps", func(t *testing.T) {
		d := t.TempDir()
		var files [][2]string
		for i := 0; i < 30; i++ {
			files = append(files, [2]string{"doc/p" + string(rune('a'+i%26)) + strings.Repeat("x", i) + ".html", "p"})
		}
		writeJar(t, filepath.Join(d, "inst", "modules", "docMany-doc.jar"), files...)
		py, gol := covCopies(t, covBase(t, d))
		covBoth(t, "many gaps", "docjars",
			[]string{covRepo(t, filepath.Join(d, "p"), "1.2", filepath.Join(d, "inst"), py), py, "1.2"},
			[]string{covRepo(t, filepath.Join(d, "g"), "1.2", filepath.Join(d, "inst"), gol), gol, "1.2"}, py, gol)
	})

	t.Run("pass and guide gap", func(t *testing.T) {
		d := t.TempDir()
		writeJar(t, filepath.Join(d, "inst", "modules", "docOne-doc.jar"),
			[2]string{"doc/one.html", "o"}, [2]string{"doc/one.png", "p"}, [2]string{"META-INF/MANIFEST.MF", "m"})
		src := filepath.Join(d, "base")
		writeFile(t, filepath.Join(src, "guides", "One", "one.html"), []byte("o"))
		writeFile(t, filepath.Join(src, "guides-clean", "One", "one.txt"), []byte("o"))
		py, gol := covCopies(t, src)
		args := func(side, base string) []string {
			return []string{covRepo(t, filepath.Join(d, side), "1.2", filepath.Join(d, "inst"), base), base, "1.2"}
		}
		pyArgs, goArgs := args("p", py), args("g", gol)
		covBoth(t, "pass", "docjars", pyArgs, goArgs, py, gol)
		for _, b := range []string{py, gol} {
			writeFile(t, filepath.Join(b, "guides", "Two", "two.html"), []byte("t"))
		}
		covBoth(t, "guide gap", "docjars", pyArgs, goArgs, py, gol)
	})

	t.Run("no indexes", func(t *testing.T) {
		py, gol := covCopies(t, covBase(t, t.TempDir()))
		for _, b := range []string{py, gol} {
			if err := os.RemoveAll(filepath.Join(b, "indexes")); err != nil {
				t.Fatal(err)
			}
		}
		covBoth(t, "no indexes", "docjars", []string{pyRepo, py, "4.99.1.2"}, []string{goRepo, gol, "4.99.1.2"}, py, gol)
	})

	t.Run("malformed index", func(t *testing.T) {
		py, gol := covCopies(t, covBase(t, t.TempDir()))
		for _, b := range []string{py, gol} {
			writeFile(t, filepath.Join(b, "indexes", "source-index.json"), []byte(`{"X": {"module": "m"}}`))
		}
		covBoth(t, "malformed index", "docjars", []string{pyRepo, py, "4.99.1.2"}, []string{goRepo, gol, "4.99.1.2"}, py, gol)
	})
}
