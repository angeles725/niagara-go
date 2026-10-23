package helpbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

const foxLexicon = "#\n# Lexicon for the fox module.\n#\nhelp.guide.base=module://docDrivers/doc\n\n" +
	"nav.foxSession=Station\nnav.foxSession.description=Fox session to a Niagara station\n" +
	"stationName=Station Name\n" +
	"certExpiring=Your certificate needs to be renewed. An expired certificate may cause a warning \\\n" +
	"    when users connect.\n" +
	"nav.spy.toolTip = Spy\n" +
	"escaped\\=key : caf\\u00e9 \\t tab\n"

func TestParseLexicon(t *testing.T) {
	got := parseLexicon([]byte("\ufeffa=1\r\n! bang\n  b : two words\nc\\:d=x\\\n  y\nnovalue\nk=\n"))
	want := []lexEntry{{"a", "1"}, {"b", "two words"}, {"c:d", "xy"}, {"k", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseLexicon = %q, want %q", got, want)
	}
	if got := unescapeLexicon(`caf\u00e9 \t tab \z`); got != "café   tab z" {
		t.Errorf("unescape = %q", got)
	}
}

// A page keeps every key = value entry in file order: help keys and
// sentences as they are, plain UI labels marked "[label]".
func TestLexiconPage(t *testing.T) {
	p, ok := lexiconPage("fox-rt.jar", "fox-rt.lexicon", []byte(foxLexicon))
	want := "TITLE: fox-rt\nMODULE: fox-rt\nLEXICON: fox-rt.lexicon\n\n" +
		"[label] help.guide.base = module://docDrivers/doc\n" +
		"\f[label] nav.foxSession = Station\n" +
		"\fnav.foxSession.description = Fox session to a Niagara station\n" +
		"\f[label] stationName = Station Name\n" +
		"\fcertExpiring = Your certificate needs to be renewed. An expired certificate may cause a warning when users connect.\n" +
		"\fnav.spy.toolTip = Spy\n" +
		"\f[label] escaped=key = café   tab\n"
	if !ok || p.rel != "fox-rt/fox-rt.lexicon.txt" || p.text != want {
		t.Errorf("page %q %v:\n%s\nwant\n%s", p.rel, ok, p.text, want)
	}
	p, ok = lexiconPage("x-rt.jar", "x.lexicon", []byte("a=Name\nb=Two Words\nempty=\n"))
	if want := "TITLE: x-rt x\nMODULE: x-rt\nLEXICON: x.lexicon\n\n[label] a = Name\n\f[label] b = Two Words\n"; !ok || p.text != want {
		t.Errorf("labels-only page %v:\n%s\nwant\n%s", ok, p.text, want)
	}
	if _, ok := lexiconPage("x-rt.jar", "x.lexicon", []byte("# comment only\nempty=\n")); ok {
		t.Error("a lexicon without a non-empty entry writes a page")
	}
}

// lexiconInstall is a registered version whose install has module jars
// with lexicons: help text (fox-rt), labels only (labels-rt), a German
// translation jar, a lexicon under doc/ (extracted as a guide file, not
// read here) and a doc jar's lexicon (never read).
func lexiconInstall(t *testing.T) (root, install string) {
	t.Helper()
	dir := t.TempDir()
	install = filepath.Join(dir, "install")
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "fox-rt.jar"), [2]string{"fox-rt.lexicon", foxLexicon},
		[2]string{"doc/guide.lexicon", "a.description=In the guide folder\n"})
	testkit.WriteJar(t, filepath.Join(mods, "labels-rt.jar"), [2]string{"labels-rt.lexicon", "a=Name\n"})
	testkit.WriteJar(t, filepath.Join(mods, "niagaraLexiconDe-rt.jar"),
		[2]string{"control.lexicon", "x.description=Zeige Liste der meist verwendeten Pfade\n"})
	testkit.WriteJar(t, filepath.Join(mods, "docFoo-doc.jar"), [2]string{"docFoo-doc.lexicon", "a.description=doc jar\n"})
	testkit.WriteJar(t, filepath.Join(mods, "docDeveloper-rt.jar"),
		[2]string{"docDeveloper-rt.lexicon", "x.description=Lexicon of an unmapped doc jar entry\n"})
	repo := filepath.Join(dir, "repo")
	root = filepath.Join(repo, "versions", "1.0")
	reg, _ := json.Marshal(map[string]any{"default": "1.0", "versions": map[string]any{"1.0": map[string]string{
		"label": "fixture", "install_dir": install, "data_root": "versions/1.0"}}})
	testkit.WriteFile(t, filepath.Join(repo, index.RegistryRel), reg)
	if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, install
}

func TestBuildLexiconText(t *testing.T) {
	root, install := lexiconInstall(t)
	res, err := buildLexiconText(root)
	if err != nil {
		t.Fatal(err)
	}
	pages := copiedFiles(t, filepath.Join(root, lexiconTextDir))
	if want := []string{"docDeveloper-rt/docDeveloper-rt.lexicon.txt", "fox-rt/fox-rt.lexicon.txt",
		"labels-rt/labels-rt.lexicon.txt", "niagaraLexiconDe-rt/control.lexicon.txt"}; !reflect.DeepEqual(pages, want) {
		t.Errorf("pages %q, want %q", pages, want)
	}
	line := "lexicon-text/: 4 lexicon files with entries of 4, 5 help and 5 label rows (from " + install + "/modules, 5 jars)"
	if res.Files != 4 || !reflect.DeepEqual(res.Log, []string{line}) {
		t.Errorf("result %+v\nwant log %q", res, line)
	}
	// The pages are a full-text source: find reaches them.
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	docs, err := fulltextDocsAt(fts.DBPath(root))
	if err != nil || !docs["lexicon-text/fox-rt/fox-rt.lexicon.txt"] || !docs["lexicon-text/niagaraLexiconDe-rt/control.lexicon.txt"] {
		t.Errorf("full-text docs %v (%v)", docs, err)
	}
	// An install that is not mounted keeps the last copy.
	if err := os.RemoveAll(filepath.Join(install, "modules")); err != nil {
		t.Fatal(err)
	}
	if res, err := buildLexiconText(root); err != nil || len(copiedFiles(t, filepath.Join(root, lexiconTextDir))) != 4 ||
		!strings.Contains(res.Log[0], "skipped (the last copy is kept)") {
		t.Errorf("unmounted install: %+v %v", res, err)
	}
}

// coverage --sweep sees the lexicons (doc_like .lexicon): a module jar's
// lexicon is indexed through its lexicon-text/ page (labels included),
// excluded when it holds no entry, and UNCLASSIFIED when its page is not
// indexed.
func TestSweepClassifiesLexicons(t *testing.T) {
	root, install := lexiconInstall(t)
	// A module copy outside modules/ (software database) that differs
	// from the installed jar is excluded by rule, not read.
	testkit.WriteJar(t, filepath.Join(install, "sw", "1.0", "fox-rt.jar"), [2]string{"fox-rt.lexicon", foxLexicon + "x.description=newer\n"})
	if !mustDefault(t).IsDocLike("x/fox-rt.lexicon") {
		t.Fatal(".lexicon is not documentation-like")
	}
	if _, err := buildLexiconText(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, lexiconTextDir, "niagaraLexiconDe-rt", "control.lexicon.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	docs := func(string) (map[string]bool, error) { return fulltextDocsAt(fts.DBPath(root)) }
	res := scanOK(t, install, root, SweepOptions{Fulltext: docs})
	got := map[string]int{}
	for k, n := range res.Reasons {
		got[k.Rule] += n
	}
	if got["lexicon-labels"] != 0 || got["lexicon-empty"] != 0 || got["lexicon-outside-modules"] != 1 {
		t.Errorf("labels-only lexicon excluded or sw/ lexicon not excluded: %v", got)
	}
	var gaps []string
	for _, g := range res.Unclassified {
		gaps = append(gaps, g.Jar+" "+g.Entry+" -- "+g.Why)
	}
	want := []string{"modules/niagaraLexiconDe-rt.jar control.lexicon -- lexicon help text is not indexed " +
		"(lexicon-text/niagaraLexiconDe-rt/control.lexicon.txt): run rebuild-all, then find"}
	if !reflect.DeepEqual(gaps, want) {
		t.Errorf("unclassified %q\nwant %q", gaps, want)
	}
	if row := res.Rows["modules/"]; row == nil || row.Indexed < 2 {
		t.Errorf("modules/ row %+v: fox-rt.lexicon and labels-rt.lexicon are indexed", row)
	}
}

// The lexicon pages are a rebuild-all step.
func TestRebuildPlanHasLexiconText(t *testing.T) {
	for _, b := range rebuildBuilders {
		if b.Script == lexiconScript && reflect.DeepEqual(b.Produces, []string{"lexicon-text/"}) {
			return
		}
	}
	t.Error("rebuild-all has no lexicon_extractor step")
}
