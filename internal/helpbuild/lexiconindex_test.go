package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

// lexiconFixtureInstall is an install whose modules/ holds a base module with
// its lexicon (an empty value, a continued line and a \u escape included), a
// French translation of it, a jar whose module.xml is malformed, a module
// without a lexicon and a file that is not a jar.
func lexiconFixtureInstall(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"),
		[2]string{"META-INF/module.xml", `<module name="kitControl-rt" moduleName="kitControl" runtimeProfile="rt"/>`},
		[2]string{"kitControl-rt.lexicon", "# comment\nBooleanLatch.displayName=Boolean Latch\nempty=\nlong=one \\\n  two\nunit=\\u00b0C\n"})
	testkit.WriteJar(t, filepath.Join(mods, "niagaraLexiconFr-rt.jar"),
		[2]string{"META-INF/module.xml", `<module name="niagaraLexiconFr-rt" moduleName="niagaraLexiconFr"><lexicons>
<lexicon module="kitControl" resource="kitControl.lexicon" language="fr" default="true"/></lexicons></module>`},
		[2]string{"kitControl.lexicon", "BooleanLatch.displayName=Verrou booléen\n"})
	testkit.WriteJar(t, filepath.Join(mods, "broken-rt.jar"),
		[2]string{"META-INF/module.xml", `<module name="broken-rt"><types></module>`},
		[2]string{"broken-rt.lexicon", "a=b\n"})
	testkit.WriteJar(t, filepath.Join(mods, "plain-rt.jar"), [2]string{"META-INF/module.xml", `<module name="plain-rt"/>`})
	for name, data := range map[string]string{"corrupt-rt.jar": "not a zip", "README.txt": "not a jar"} {
		if err := os.WriteFile(filepath.Join(mods, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return install
}

// TestLexiconIndexReadsEveryEntry: every lexicon file is a files row naming
// its module, its language and the base module a translation overrides, with
// every key = value entry in file order; an unreadable jar or descriptor is an
// errors row and a warning (a lexicon whose descriptor is unreadable is still
// indexed, its module named after the jar).
func TestLexiconIndexReadsEveryEntry(t *testing.T) {
	install, root := lexiconFixtureInstall(t), t.TempDir()
	res, err := buildLexiconIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, lexiconIndexName))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := idx.Get("jars"); func() int64 { n, _ := index.Int(v); return n }() != 5 {
		t.Errorf("jars = %v, want every jar of modules/", v)
	}
	files := strings.Join(rows(t, idx, "files"), "\n")
	wantFiles := `{"jar": "broken-rt.jar", "path": "broken-rt.lexicon", "module": "broken", "language": "", "overrides": "", "entries": [["a", "b"]]}
{"jar": "kitControl-rt.jar", "path": "kitControl-rt.lexicon", "module": "kitControl", "language": "", "overrides": "", "entries": [["BooleanLatch.displayName", "Boolean Latch"], ["empty", ""], ["long", "one two"], ["unit", "\u00b0C"]]}
{"jar": "niagaraLexiconFr-rt.jar", "path": "kitControl.lexicon", "module": "niagaraLexiconFr", "language": "fr", "overrides": "kitControl", "entries": [["BooleanLatch.displayName", "Verrou bool\u00e9en"]]}`
	if files != wantFiles {
		t.Errorf("files:\n%s", files)
	}
	errs := strings.Join(rows(t, idx, "errors"), "\n")
	for _, want := range []string{`{"jar": "broken-rt.jar", "path": "META-INF/module.xml", "error": "XML syntax error`,
		`{"jar": "corrupt-rt.jar", "path": "", "error": `} {
		if !strings.Contains(errs, want) {
			t.Errorf("errors lack %s:\n%s", want, errs)
		}
	}
	if len(res.Warnings) != 2 || !strings.HasPrefix(res.Warnings[0], "WARNING: lexicon_index_builder: broken-rt.jar!META-INF/module.xml: ") {
		t.Errorf("warnings = %q", res.Warnings)
	}
	want := "lexicon-index.json: 3 lexicon files (2 base, 1 translations) of 2 modules in 1 languages," +
		" 6 entries (5 distinct module keys) from 5 jars (2 errors) -> "
	if res.Files != 3 || len(res.Log) != 1 || !strings.HasPrefix(res.Log[0], want) {
		t.Errorf("files %d log %q", res.Files, res.Log)
	}
}

// TestLexiconIndexNeedsARegisteredInstall: the rebuild-all step writes nothing
// on a data root no registry names, and says why.
func TestLexiconIndexNeedsARegisteredInstall(t *testing.T) {
	root := t.TempDir()
	res, err := buildLexiconIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "lexicon-index.json: no install registered; skipped (the last copy is kept)"; len(res.Log) != 1 || res.Log[0] != want {
		t.Errorf("log = %q", res.Log)
	}
	if _, err := os.Stat(index.Path(root, lexiconIndexName)); !os.IsNotExist(err) {
		t.Errorf("lexicon-index.json written: %v", err)
	}
}
