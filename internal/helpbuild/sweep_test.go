package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/index"
)

// The whole-install sweep on the fixture of niagara-help
// tests/test_install_sweep.py (plus the test_t3a / test_n3 corners),
// without Python: the data root is extracted by Extract and its clean
// pages are written by hand, so each case states what makes an entry
// indexed. testdata/parity/sweepdriver.py compares the full reports.

const sweepPage = "<html><head><title>%s</title></head><body><p>%s</p></body></html>"

func page(title, body string) string {
	return strings.Replace(strings.Replace(sweepPage, "%s", title, 1), "%s", body, 1)
}

var sweepHelpJar = [][2]string{
	{"doc/About-Help.html", page("About Help", "helpzork page")},
	{"com/acme/BHelp.class", "\xca\xfe"},
}

// sweepTestRules is the test's RULES plus the shipped extract rules and
// globs (the extraction used them through DestPath).
func sweepTestRules(t *testing.T) *Rules {
	t.Helper()
	var shipped map[string]any
	if err := json.Unmarshal(embeddedRules, &shipped); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{
		"doc_like": map[string]any{"extensions": []string{".html", ".htm", ".md", ".txt", ".pdf"},
			"names": []string{"toc.xml"}},
		"skip_dirs": []any{map[string]string{"dir": "jre", "reason": "bundled Java runtime"}},
		"extract":   shipped["extract"],
		"globs":     shipped["globs"],
		"exclude": []any{
			map[string]string{"id": "node-modules", "entry": "**/node_modules/**", "reason": "bundled npm package"},
			map[string]string{"id": "third-party-java", "entry": "org/**", "reason": "third-party library resource"},
			map[string]string{"id": "docs-menu", "jar": "(docs)", "entry": "menu/**", "reason": "PDF library menu frame"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(data, "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// sweepInstallFixture is make_repo: the install, a registered data root (1.0)
// extracted and made searchable, and the loose docs/ files with their
// pdf-index.json row.
func sweepInstallFixture(t *testing.T) (repo, root, install string) {
	t.Helper()
	tmp := t.TempDir()
	install = filepath.Join(tmp, "install")
	mods := filepath.Join(install, "modules")
	writeJar(t, filepath.Join(mods, "docGuide-doc.jar"),
		[2]string{"doc/index.html", page("Guide", "guide home")},
		[2]string{"doc/Intro.html", page("Intro", "intro text")},
		[2]string{"doc/LICENSE.txt", "licence text\n"})
	writeJar(t, filepath.Join(mods, "helpMod-rt.jar"), sweepHelpJar...)
	writeJar(t, filepath.Join(mods, "thing-ux.jar"),
		[2]string{"rc/notes.md", "# Notes\n\nquibbleNotes of the thing module\n"},
		[2]string{"rc/node_modules/pkg/README.md", "npm readme\n"},
		[2]string{"rc/thing.js", "var x;"})
	writeJar(t, filepath.Join(mods, "lib-rt.jar"),
		[2]string{"org/apache/words.txt", "a\nb\n"},
		[2]string{"com/acme/BLib.class", "\xca\xfe"})
	writeJar(t, filepath.Join(install, "sw", "1.0", "helpMod-rt.jar"), sweepHelpJar...)
	writeJar(t, filepath.Join(install, "sw", "2.0", "helpMod-rt.jar"),
		[2]string{"doc/About-Help.html", page("About Help", "newer text")})
	writeJar(t, filepath.Join(install, "jre", "lib", "rt.jar"), [2]string{"about.html", "x"})

	repo = filepath.Join(tmp, "repo")
	root = filepath.Join(repo, "versions", "1.0")
	reg, _ := json.Marshal(map[string]any{"default": "1.0", "versions": map[string]any{"1.0": map[string]string{
		"label": "fixture", "install_dir": install, "data_root": "versions/1.0"}}})
	writeFile(t, filepath.Join(repo, index.RegistryRel), reg)
	if _, err := Extract(install, root, NewListings("")); err != nil {
		t.Fatal(err)
	}
	for _, clean := range []string{"Guide/index.txt", "Guide/Intro.txt", "helpMod-rt/About-Help.txt", "thing-ux/rc/notes.md.txt"} {
		writeFile(t, filepath.Join(root, "guides-clean", filepath.FromSlash(clean)), []byte("TITLE: x\n"))
	}
	writeFile(t, filepath.Join(install, "docs", "menu", "tree.html"), []byte(page("Docs", "menu")))
	writeFile(t, filepath.Join(install, "docs", "Manual.pdf"), []byte("%PDF-1.4 not really"))
	writeFile(t, filepath.Join(root, "indexes", "pdf-index.json"), []byte(`{"Manual": {"source_pdf": "Manual.pdf"}}`))
	return repo, root, install
}

func scanOK(t *testing.T, install, root string, opts SweepOptions) *SweepResult {
	t.Helper()
	res, err := Scan(install, root, opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func ruleCounts(res *SweepResult) map[string]int {
	out := map[string]int{}
	for k, n := range res.Reasons {
		out[k.Rule] += n
	}
	return out
}

func gapKeys(res *SweepResult) []string {
	var out []string
	for _, g := range res.Unclassified {
		out = append(out, g.Jar+" "+g.Entry)
	}
	return out
}

// Every documentation-like entry is indexed, excluded with its rule, or
// UNCLASSIFIED; a copy outside modules/ is excluded only while it is
// byte-identical; loose PDFs are indexed through pdf-index.json; skipped
// directories are reported, never walked.
func TestSweepClassifiesEveryEntry(t *testing.T) {
	_, root, install := sweepInstallFixture(t)
	res := scanOK(t, install, root, SweepOptions{Rules: sweepTestRules(t)})
	if got := res.Rows["modules/"].Indexed; got != 4 {
		t.Errorf("modules/ indexed = %d, want 4 (index, Intro, the help page, rc/notes.md)", got)
	}
	want := map[string]int{"node-modules": 1, "third-party-java": 1, "docs-menu": 1, "doc_types": 1, "installed-copy": 1}
	if got := ruleCounts(res); !mapsEqual(got, want) {
		t.Errorf("reasons by rule = %v, want %v", got, want)
	}
	if got := gapKeys(res); !slices.Equal(got, []string{"sw/2.0/helpMod-rt.jar doc/About-Help.html"}) {
		t.Errorf("unclassified = %q", got)
	}
	loose := res.Rows["docs/ (loose files)"]
	if loose.DocLike != 2 || loose.Indexed != 1 || loose.Excluded != 1 {
		t.Errorf("loose docs row = %+v, want 2 doc-like, 1 indexed, 1 excluded", *loose)
	}
	if !slices.Equal(res.Skipped, []SkipDir{{"jre", "bundled Java runtime"}}) || res.Rows["jre/"] != nil {
		t.Errorf("skipped = %v, jre/ row %v", res.Skipped, res.Rows["jre/"])
	}
	if res.Jars != 6 || res.Listed != 6 {
		t.Errorf("jars %d listed %d, want 6 and 6", res.Jars, res.Listed)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// The gate prints the whole report and exits 1 on an unclassified entry;
// an unavailable install exits 2 with one line.
func TestRunSweepReport(t *testing.T) {
	repo, root, install := sweepInstallFixture(t)
	var buf bytes.Buffer
	status, err := RunSweep(repo, root, "1.0", &buf, SweepOptions{Rules: sweepTestRules(t)})
	if err != nil || status != SweepFail {
		t.Fatalf("status %d, err %v", status, err)
	}
	want := `INSTALL SWEEP 1.0 (6 jars under <install>; 6 listed anew, the rest from the jar-listing cache)
  documentation-like: .html .htm .md .txt .pdf + toc.xml
  location                       jars with-docs  doc-like  indexed excluded UNCLASSIFIED
  docs/ (loose files)               0         0         2        1        1            0
  modules/                          4         4         7        4        3            0
  sw/                               2         2         2        0        1            1
  TOTAL                             6         6        11        5        5            1

SKIPPED DIRECTORIES (install_docs.json skip_dirs):
  jre/  bundled Java runtime

EXCLUDED (rule: reason):
        1  doc_types: licence text
        1  docs-menu: PDF library menu frame
        1  installed-copy: byte-identical (path, size, CRC-32) to the entry of an installed jar, classified there
        1  node-modules: bundled npm package
        1  third-party-java: third-party library resource

PDF MANUALS: none (no registered version has docs-text/)

COURSE: none bound to this version

UNCLASSIFIED (first 1 of 1):
  sw/2.0/helpMod-rt.jar  doc/About-Help.html  -- documentation in a jar outside modules/ that the installed copy does not have (not extracted)
RESULT: FAIL - 1 unclassified entry
`
	if got := strings.ReplaceAll(buf.String(), install, "<install>"); got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
	cache, _ := CachePath(repo, install)
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("the gate keeps the listings in the repository cache: %v", err)
	}

	buf.Reset()
	status, err = RunSweep(repo, root, "9.9", &buf, SweepOptions{})
	if status != SweepUnavailable || err != nil ||
		buf.String() != "INSTALL SWEEP 9.9: install not available (?); cannot read the jars, gate not evaluated.\n" {
		t.Errorf("unknown version: %d %v %q", status, err, buf.String())
	}
}

// Listings are cached by path + size + mtime: a second sweep opens no
// jar, a rewritten jar is listed again.
func TestSweepListingCache(t *testing.T) {
	_, root, install := sweepInstallFixture(t)
	rules := sweepTestRules(t)
	path := filepath.Join(t.TempDir(), "cache", "listings.json")
	if first := scanOK(t, install, root, SweepOptions{Rules: rules, Listings: NewListings(path)}); first.Listed == 0 {
		t.Fatal("first sweep listed nothing")
	}
	if second := scanOK(t, install, root, SweepOptions{Rules: rules, Listings: NewListings(path)}); second.Listed != 0 {
		t.Errorf("second sweep listed %d jars, want 0", second.Listed)
	}
	jar := filepath.Join(install, "modules", "lib-rt.jar")
	writeJar(t, jar, [2]string{"org/apache/words.txt", "a\nb\nc\n"})
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(jar, later, later); err != nil {
		t.Fatal(err)
	}
	if third := scanOK(t, install, root, SweepOptions{Rules: rules, Listings: NewListings(path)}); third.Listed != 1 {
		t.Errorf("after rewriting one jar: listed %d, want 1", third.Listed)
	}
}

// t3aRules is test_t3a_advisories._rules.
func t3aRules(t *testing.T, extract ...map[string]string) *Rules {
	t.Helper()
	data, _ := json.Marshal(map[string]any{
		"doc_like":  map[string]any{"extensions": []string{".html", ".md", ".txt", ".pdf"}, "names": []string{"toc.xml"}, "dirs": []string{}},
		"skip_dirs": []any{}, "extract": append([]map[string]string{}, extract...), "exclude": []any{}})
	r, err := Parse(data, "rules")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// An unreadable directory fails the gate (once, even when both walks meet
// it under docs/); .jar and loose PDF names match in any letter case; the
// sweep's rules reach DestPath; a corrupt jar is reported unreadable.
func TestSweepCorners(t *testing.T) {
	tmp := t.TempDir()
	install, root := filepath.Join(tmp, "install"), filepath.Join(tmp, "data")
	writeJar(t, filepath.Join(install, "sw", "locked", "x.jar"), [2]string{"doc/a.html", "<p>a"})
	writeJar(t, filepath.Join(install, "modules", "Upper.JAR"), [2]string{"doc/a.html", "<p>a"})
	writeJar(t, filepath.Join(install, "modules", "mod-rt.jar"), [2]string{"notes/a.txt", "notes"})
	writeFile(t, filepath.Join(install, "modules", "broken-rt.jar"), []byte("PK\x03\x04 not a zip"))
	writeFile(t, filepath.Join(install, "modules", "empty-rt.jar"), nil)
	writeFile(t, filepath.Join(install, "docs", "sub", "Manual.PDF"), []byte("%PDF-1.4"))
	writeFile(t, filepath.Join(install, "docs", "locked", "Other.pdf"), []byte("%PDF-1.4"))
	writeFile(t, filepath.Join(root, "indexes", "pdf-index.json"), []byte(`{"Manual": {"source_pdf": "sub/manual.pdf"}}`))
	locked := lockDir(t, filepath.Join(install, "sw", "locked"))
	locked = lockDir(t, filepath.Join(install, "docs", "locked")) && locked

	jars, _, _ := FindJars(install, t3aRules(t))
	var rels []string
	for _, j := range jars {
		rels = append(rels, j.Rel)
	}
	if want := []string{"modules/Upper.JAR", "modules/broken-rt.jar", "modules/empty-rt.jar", "modules/mod-rt.jar"}; !slices.Equal(rels, want) {
		t.Errorf("jars = %q, want %q", rels, want)
	}

	notes := map[string]string{"id": "notes", "entry": "notes/*.txt", "reason": "module notes (test rule)"}
	res := scanOK(t, install, root, SweepOptions{Rules: t3aRules(t, notes)})
	if got := res.Rows["docs/ (loose files)"].Indexed; got != 1 {
		t.Errorf("upper-case loose PDF: indexed %d, want 1", got)
	}
	if !slices.Equal(res.Unreadable, []string{"modules/broken-rt.jar"}) {
		t.Errorf("unreadable = %q", res.Unreadable)
	}
	if res.Reasons[ReasonKey{"empty-file", "empty file named .jar (0 bytes, nothing to list)"}] != 1 {
		t.Errorf("empty jar not excluded: %v", res.Reasons)
	}
	whys := map[string]string{}
	for _, g := range res.Unclassified {
		whys[g.Jar+" "+g.Entry] = g.Why
	}
	if w := whys["modules/mod-rt.jar notes/a.txt"]; !strings.HasPrefix(w, "maps to guides/mod-rt/notes/") {
		t.Errorf("the sweep's extract rule did not reach DestPath: %q", w)
	}
	if !locked {
		t.Skip("running with permissions that read mode-0 folders")
	}
	for _, d := range []string{"sw/locked", "docs/locked"} {
		if w := whys[d+" (directory)"]; w != "unreadable directory, not swept: Permission denied" {
			t.Errorf("%s: %q", d, w)
		}
	}
	n := 0
	for _, g := range res.Unclassified {
		if g.Jar == "docs/locked" {
			n++
		}
	}
	if n != 1 || res.Rows["docs/"].Unclassified != 1 {
		t.Errorf("docs/locked reported %d times (docs/ row %+v), want once", n, res.Rows["docs/"])
	}
}

// Two entries of one installed jar that map to one corpus file: the
// extractor writes the first; the second is excluded when byte-identical,
// UNCLASSIFIED otherwise (doc_jars.mapped_entries is the shared rule).
func TestSweepShadowedEntries(t *testing.T) {
	for _, c := range []struct {
		second                       string
		indexed, excluded, unclassed int
	}{{"# first\n", 1, 1, 0}, {"# frist\n", 1, 0, 1}} { // same size: the CRC decides
		tmp := t.TempDir()
		install, root := filepath.Join(tmp, "install"), filepath.Join(tmp, "data")
		writeJar(t, filepath.Join(install, "modules", "mmm-ux.jar"),
			[2]string{"doc/rc/x.md", "# first\n"}, [2]string{"rc/x.md", c.second})
		writeFile(t, filepath.Join(root, "guides", "mmm-ux", "rc", "x.md"), []byte("# first\n"))
		writeFile(t, filepath.Join(root, "guides-clean", "mmm-ux", "rc", "x.md.txt"), []byte("TITLE: first\n"))
		res := scanOK(t, install, root, SweepOptions{})
		row := res.Rows["modules/"]
		if row.Indexed != c.indexed || row.Excluded != c.excluded || row.Unclassified != c.unclassed {
			t.Errorf("second %q: row %+v", c.second, *row)
		}
		if c.unclassed == 1 && (len(res.Unclassified) != 1 || !strings.Contains(res.Unclassified[0].Why, "doc/rc/x.md")) {
			t.Errorf("gap does not name the earlier entry: %v", res.Unclassified)
		}
	}
}

func TestSweepLocation(t *testing.T) {
	for rel, want := range map[string]string{"x.jar": "(root)", "modules/a.jar": "modules/",
		"sw/1.0/a.jar": "sw/", "./x": "./"} {
		if got := sweepLocation(rel); got != want {
			t.Errorf("%q: %q, want %q", rel, got, want)
		}
	}
	for rel, want := range map[string]bool{"modules/a.jar": true, "javadoc/j.jar": true,
		"sw/a.jar": false, "modules/x/a.jar": false, "a.jar": false} {
		if got := isInstalledJar(rel); got != want {
			t.Errorf("isInstalledJar(%q) = %v", rel, got)
		}
	}
}

func TestReportFormatting(t *testing.T) {
	long := strings.Repeat("é", 30)
	got := sweepRowLine((&SweepRow{Jars: 1234}).cells(firstRunes(long, 28))...)
	if want := "  " + strings.Repeat("é", 28) + "  1,234         0         0        0        0            0"; got != want {
		t.Errorf("row:\n%q\nwant\n%q", got, want)
	}
	m := map[ReasonKey]int{{"b", "x"}: 2, {"a", "y"}: 2, {"a", "x"}: 2, {"z", "z"}: 5}
	if got := sortedReasons(m); !slices.Equal(got, []ReasonKey{{"z", "z"}, {"a", "x"}, {"a", "y"}, {"b", "x"}}) {
		t.Errorf("reason order %v", got)
	}
}

// The sweep's default full-text reader uses only an up-to-date index:
// missing, or stale after a text file was added, is an error rather than a
// rebuild (open_index's building is the caller's FulltextDocs).
func TestFreshFulltextDocs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "devguide-examples", "x", "demo.js"), []byte("function demo() {}\n"))
	var stale *fts.StaleIndexError
	if _, err := FreshFulltextDocs(root); !errors.As(err, &stale) || !stale.Missing {
		t.Errorf("no index: %v", err)
	}
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	docs, err := FreshFulltextDocs(root)
	if err != nil || !docs["devguide-examples/x/demo.js"] {
		t.Errorf("fresh index: %v %v", docs, err)
	}
	writeFile(t, filepath.Join(root, "devguide-examples", "x", "late.js"), []byte("function late() {}\n"))
	if _, err := FreshFulltextDocs(root); !errors.As(err, &stale) || stale.Missing {
		t.Errorf("stale index: %v", err)
	}
}
