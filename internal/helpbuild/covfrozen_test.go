package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The coverage reports (Coverage, printGuideCoverage) and the doc-jar gate
// (DocJarCoverage) as two lines per run, the printed bytes and the outcome,
// checked against their frozen Python output (frozen_test.go) on the cases
// a golden file does not hold: every report mode, empty and missing
// indexes, the malformed indexes Python read without error, and every gate
// outcome.

// covResult renders a Go run as two lines: ["out", text] and the status,
// ["missing", message] for a *MissingIndexError or ["raise"].
func covResult(out string, status pyjson.Value, err error) []string {
	last := testkit.DumpLine(testkit.Str("status"), status)
	var missing *MissingIndexError
	switch {
	case errors.As(err, &missing):
		last = testkit.DumpLine(testkit.Str("missing"), testkit.Str(err.Error()))
	case err != nil:
		last = testkit.DumpLine(testkit.Str("raise"))
	}
	return []string{testkit.DumpLine(testkit.Str("out"), testkit.Str(out)), last}
}

// goCov runs one mode: coverage <base> [module], guides <base>, or docjars
// <repo> <base> [version].
func goCov(mode string, args ...string) []string {
	var b strings.Builder
	switch mode {
	case "coverage":
		module := ""
		if len(args) > 1 {
			module = args[1]
		}
		res, err := Coverage(&b, args[0], module)
		status := pyjson.Value{}
		switch {
		case res.Miss:
			b.WriteString(testkit.DumpLine(testkit.Str("miss"), testkit.Str("coverage"), testkit.Str(module), testkit.Str("modules")) + "\n")
		case module == "":
			status = testkit.IntV(res.GuideGaps)
		}
		return covResult(b.String(), status, err)
	case "guides":
		n, err := printGuideCoverage(&b, args[0])
		return covResult(b.String(), testkit.IntV(n), err)
	case "docjars":
		return goCovDocJars(buildingDocs, args...)
	}
	panic("unknown mode " + mode)
}

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
	return fulltextDocsAt(path)
}

// goCovDocJars is the docjars mode: repo, base, optional version.
func goCovDocJars(docs FulltextDocs, args ...string) []string {
	var b strings.Builder
	vid := ""
	if len(args) > 2 {
		vid = args[2]
	}
	st, err := DocJarCoverage(&b, args[0], args[1], vid, docs)
	return covResult(b.String(), testkit.IntV(st), err)
}

// covFrozen runs mode with args and checks its lines, base shown as
// <base>, against testdata/frozen/cov/<name>.txt.
func covFrozen(t *testing.T, name, mode string, args []string, base string) {
	t.Helper()
	testkit.CheckFrozen(t, "cov/"+name, testkit.Relabeled(goCov(mode, args...), base, "<base>"))
}

// covCopy copies the data root src to a new temporary folder.
func covCopy(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, src, dst)
	return dst
}

// TestCoverageReportFrozen: the overview, module reports (a prefix match
// over rt/ux/wb, undocumented classes with and without a package, a module
// that matches nothing), the guide coverage alone, an empty and a missing
// index.
func TestCoverageReportFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := covCopy(t, covReportBase(t, t.TempDir()))
	for _, c := range [][]string{
		{"coverage"}, {"coverage", "alarm-rt"}, {"coverage", "ALARM"}, {"coverage", "alarm-ux"}, {"coverage", "zeta-rt"},
		{"coverage", "école"}, {"coverage", "rt"}, {"coverage", "nothing"}, {"guides"},
	} {
		what := strings.Join(c, " ")
		covFrozen(t, "report/"+testkit.FrozenName(what), c[0], append([]string{base}, c[1:]...), base)
	}
	empty := covCopy(t, base)
	testkit.WriteFile(t, filepath.Join(empty, "indexes", "source-index.json"), []byte(`{}`))
	if err := os.RemoveAll(filepath.Join(empty, "guides")); err != nil {
		t.Fatal(err)
	}
	covFrozen(t, "report/empty", "coverage", []string{empty}, empty)
	for _, name := range []string{"class-index.json", "method-index.json", "slots-index.json"} {
		b := covCopy(t, base)
		if err := os.Remove(filepath.Join(b, "indexes", name)); err != nil {
			t.Fatal(err)
		}
		covFrozen(t, "report/missing-"+name, "coverage", []string{b, "alarm"}, b)
	}
}

// TestCoverageMalformedFrozen: on the malformed indexes Python read
// without error and Go accepts too (covMalformed, python "ok", shape
// false), the Go report is Python's byte for byte.
func TestCoverageMalformedFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	src := covReportBase(t, t.TempDir())
	for _, c := range covMalformed {
		if c.python != "ok" || c.shape {
			continue
		}
		b := covCopy(t, src)
		testkit.WriteFile(t, filepath.Join(b, "indexes", c.file), []byte(c.text))
		args := []string{b}
		if c.module != "" {
			args = append(args, c.module)
		}
		covFrozen(t, "malformed/"+testkit.FrozenName(c.name), "coverage", args, b)
	}
}

// TestDocJarCoverageFrozen: the gate on the synthetic install, by version
// id and identified from the data root; with more than firstGaps gaps; a
// clean install (PASS) and one whose only failure is a guide folder; an
// install that is not there, an unregistered version, a data root without
// indexes or with a malformed one.
func TestDocJarCoverageFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	inst := covInstall(t, dir)
	base := covCopy(t, covBase(t, dir))
	repo := covRepo(t, filepath.Join(dir, "gorepo"), "4.99.1.2", inst, base)
	gate := func(name string, args []string, base string) {
		t.Helper()
		covFrozen(t, "docjars/"+name, "docjars", args, base)
	}
	gate("by-version", []string{repo, base, "4.99.1.2"}, base)
	gate("unregistered-version", []string{repo, base, "4.0.0.0"}, base)
	gate("no-install_dir", []string{repo, base, "9.9.9.9"}, base)
	gate("no-registry", []string{dir, base, "4.99.1.2"}, base)

	// identified from the data root: a versions/<id>/ root two levels
	// under the repository that registers it
	idRepo := filepath.Join(t.TempDir(), "go")
	root := filepath.Join(idRepo, "versions", "4.99.1.2")
	testkit.CopyTree(t, base, root)
	testkit.WriteFile(t, filepath.Join(idRepo, "versions", "registry.json"), []byte(
		`{"versions": {"4.99.1.2": {"install_dir": "`+inst+`", "data_root": "versions/4.99.1.2"}}}`))
	gate("identified", []string{idRepo, root}, root)
	gate("not-identified", []string{dir, base}, base)

	d := t.TempDir()
	var files [][2]string
	for i := 0; i < 30; i++ {
		files = append(files, [2]string{"doc/p" + string(rune('a'+i%26)) + strings.Repeat("x", i) + ".html", "p"})
	}
	testkit.WriteJar(t, filepath.Join(d, "inst", "modules", "docMany-doc.jar"), files...)
	many := covCopy(t, covBase(t, d))
	gate("many-gaps", []string{covRepo(t, filepath.Join(d, "g"), "1.2", filepath.Join(d, "inst"), many), many, "1.2"}, many)

	d = t.TempDir()
	testkit.WriteJar(t, filepath.Join(d, "inst", "modules", "docOne-doc.jar"),
		[2]string{"doc/one.html", "o"}, [2]string{"doc/one.png", "p"}, [2]string{"META-INF/MANIFEST.MF", "m"})
	src := filepath.Join(d, "base")
	testkit.WriteFile(t, filepath.Join(src, "guides", "One", "one.html"), []byte("o"))
	testkit.WriteFile(t, filepath.Join(src, "guides-clean", "One", "one.txt"), []byte("o"))
	one := covCopy(t, src)
	args := []string{covRepo(t, filepath.Join(d, "g"), "1.2", filepath.Join(d, "inst"), one), one, "1.2"}
	gate("pass", args, one)
	testkit.WriteFile(t, filepath.Join(one, "guides", "Two", "two.html"), []byte("t"))
	gate("guide-gap", args, one)

	noIndexes := covCopy(t, covBase(t, t.TempDir()))
	if err := os.RemoveAll(filepath.Join(noIndexes, "indexes")); err != nil {
		t.Fatal(err)
	}
	gate("no-indexes", []string{repo, noIndexes, "4.99.1.2"}, noIndexes)
	malformed := covCopy(t, covBase(t, t.TempDir()))
	testkit.WriteFile(t, filepath.Join(malformed, "indexes", "source-index.json"), []byte(`{"X": {"module": "m"}}`))
	gate("malformed-index", []string{repo, malformed, "4.99.1.2"}, malformed)
}
