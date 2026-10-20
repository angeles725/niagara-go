package helpbuild

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// S2: doc-jar mapping, extraction, fill-missing, fidelity and the version
// manifest on synthetic installs, against their frozen Python output
// (frozen_test.go). Trees are listed by treeLines: every path, kind,
// permission bits, size and sha256 (manifests with generated_utc
// normalized, inline).

func collisionsValue(cs []Collision) pyjson.Value {
	a := make([]pyjson.Value, len(cs))
	for i, c := range cs {
		a[i] = testkit.Array(testkit.Str(c.Rel), testkit.Str(c.Owner), testkit.Str(c.By))
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

// extractValue is doc_jars.extract's dict, null for nil.
func extractValue(s *ExtractStats) pyjson.Value {
	if s == nil {
		return pyjson.Value{}
	}
	v := pyjson.NewObject()
	v.Set("files", testkit.IntV(s.Files))
	v.Set("bytes", uintValue(s.Bytes))
	v.Set("collisions", collisionsValue(s.Collisions))
	return v
}

// fidelityValue is doc_jars.fidelity's dict in its key order, null for nil.
func fidelityValue(r *FidelityReport) pyjson.Value {
	if r == nil {
		return pyjson.Value{}
	}
	v := pyjson.NewObject()
	v.Set("checked", testkit.IntV(r.Checked))
	v.Set("matched", testkit.IntV(r.Matched))
	v.Set("mismatched", stringsValue(r.Mismatched))
	v.Set("missing", stringsValue(r.Missing))
	v.Set("eol_only", stringsValue(r.EOLOnly))
	v.Set("collisions", collisionsValue(r.Collisions))
	v.Set("jars", testkit.IntV(r.Jars))
	return v
}

// goSetup runs ExtractAndIndex as lines: ["out", line] per log line, then
// ["result", status, error, extract stats, fidelity report] or ["raise"].
func goSetup(repo, vid, op string) []string {
	res, err := ExtractAndIndex(repo, vid, SetupOptions{Force: op == "force", VerifyOnly: op == "verify", FillMissing: op == "fill"})
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(l)))
		}
	}
	if err != nil {
		return append(out, `["raise"]`)
	}
	return append(out, testkit.DumpLine(testkit.Str("result"), testkit.IntV(res.Status), testkit.OptStr(res.Error),
		extractValue(res.Extracted), fidelityValue(res.Fidelity)))
}

// setupFrozen runs one extract_and_index on repo and checks its lines and
// the tree delta it made (prev is the tree before it) against
// testdata/frozen/extract/<name>.txt; it returns the tree after it.
func setupFrozen(t *testing.T, name, repo, vid, op string, prev []string) []string {
	t.Helper()
	lines := append(relabel(goSetup(repo, vid, op), repo), "tree:")
	cur := maskListingCache(testkit.TreeLines(t, repo))
	testkit.CheckFrozen(t, "extract/"+name, append(lines, testkit.TreeDelta(prev, cur)...))
	return cur
}

// listingCacheLine is a treeLines entry of a jar-listing cache file: its
// name hashes the install path and its bytes hold the jars' paths and
// mtimes, so a frozen tree shows only that it exists.
var listingCacheLine = regexp.MustCompile(`^(\.cache/jar-listings/)[0-9a-f]{16}(\.json -+ [0-7]+) [0-9]+ [0-9a-f]{64}$`)

// maskListingCache is lines with each listing-cache entry masked.
func maskListingCache(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = listingCacheLine.ReplaceAllString(l, "${1}<install hash>${2} <size> <sha256>")
	}
	return out
}

func relabel(lines []string, repo string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.ReplaceAll(l, repo, "<repo>")
	}
	return out
}

// TestExtractFrozen: extract_and_index on a synthetic install: an install
// that is not mounted, the repository root (refused, verify allowed), a
// first extract and a refused second one, then verify, fill and force over
// a tampered tree (mismatched, eol-only and missing files, strays).
func TestExtractFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	install := syntheticInstall(t, dir)
	repo := filepath.Join(dir, "go")
	writeRegistry(t, repo, map[string][3]string{
		"1.0.0.0": {"fixture ñ", install, "versions/1.0.0.0"},
		"1.0.0.1": {"repo root", install, "."},
		"9.9.9.9": {"not mounted", install + "-missing", "versions/9.9.9.9"},
	})
	tree := maskListingCache(testkit.TreeLines(t, repo))
	tree = setupFrozen(t, "1-not-mounted-extract", repo, "9.9.9.9", "extract", tree)
	tree = setupFrozen(t, "2-repo-root-extract", repo, "1.0.0.1", "extract", tree)
	tree = setupFrozen(t, "3-repo-root-verify", repo, "1.0.0.1", "verify", tree)
	tree = setupFrozen(t, "4-extract", repo, "1.0.0.0", "extract", tree)
	setupFrozen(t, "5-extract-again", repo, "1.0.0.0", "extract", tree)
	r := filepath.Join(repo, "versions", "1.0.0.0")
	testkit.WriteFile(t, filepath.Join(r, "guides", "ClHVACTest", "Tasks.html"), []byte("tampered"))
	testkit.WriteFile(t, filepath.Join(r, "guides", "ClHVACTest", "crlf.txt"), []byte("a\nb\n"))
	testkit.WriteFile(t, filepath.Join(r, "guides", "Stored", "s.txt"), []byte("stored\nline\n"))
	testkit.WriteFile(t, filepath.Join(r, "devguide-rc", "com", "x", "A.java"), []byte("class A {}\n"))
	for _, gone := range []string{"guides/ClHVACTest/README.txt", "devguide-rc/com/x/BVal.java", "bajadoc/index.html"} {
		if err := os.Remove(filepath.Join(r, filepath.FromSlash(gone))); err != nil {
			t.Fatal(err)
		}
	}
	testkit.WriteFile(t, filepath.Join(r, "guides", "stray.txt"), []byte("removed by --force"))
	testkit.WriteFile(t, filepath.Join(r, "indexes", "old.json"), []byte("{}"))
	testkit.WriteFile(t, filepath.Join(r, "keep.txt"), []byte("outside the raw and derived dirs"))
	tree = maskListingCache(testkit.TreeLines(t, repo))
	tree = setupFrozen(t, "6-verify", repo, "1.0.0.0", "verify", tree) // mismatched, eol-only, missing
	tree = setupFrozen(t, "7-fill", repo, "1.0.0.0", "fill", tree)     // writes only the missing
	setupFrozen(t, "8-force", repo, "1.0.0.0", "force", tree)          // wipes raw + derived dirs
}

// TestDocJarCornersFrozen: the doc-jar library calls on installs that made
// Python raise or take a corner: entry-reader errors, unreadable doc jars,
// a jar that is a folder. Per install, each call's line and the tree it
// left (or <absent>), then fidelity over what extract left.
func TestDocJarCornersFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	ok := testkit.RawEntry{Name: "doc/ok.html", Data: "ok"}
	cases := map[string][]byte{
		"badcrc":    testkit.RawZip("", []testkit.RawEntry{ok, {Name: "doc/a.txt", Data: "abc", CRCXor: 1}}, ""),
		"localname": testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "abc", LocalName: "doc/b.txt"}}, ""),
		"encrypted": testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "abc", Flags: 1}}, ""),
		"method99":  testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "abc", Method: 99}}, ""),
		"overlap":   testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "abc", CompPlus: 4}, ok}, ""),
		"shared":    testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "abc"}, {Name: "doc/a.txt", Data: "abc", SharedHeader: 1}}, ""),
		"short":     testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.txt", Data: "hello", ShortSize: 3}}, ""),
		"cp437":     testkit.RawZip("", []testkit.RawEntry{{Name: "doc/\x81ber.html", Data: "u"}, {Name: "doc/n.html\x00.exe", Data: "n"}}, ""),
		"prepended": testkit.RawZip(strings.Repeat("#", 33), []testkit.RawEntry{ok, {Name: "doc/b.txt", Data: "b"}}, ""),
		"zip64":     testkit.Zip64Zip("", []testkit.RawEntry{ok}, 1, 0),
		"notazip":   []byte("plain text"),
		"folder":    nil,
	}
	dir := t.TempDir()
	for _, name := range pyjson.SortedKeys(keysOfBytes(cases)) {
		install := filepath.Join(dir, name, "install")
		jar := filepath.Join(install, "modules", "docCase-doc.jar")
		if cases[name] == nil {
			if err := os.MkdirAll(jar, 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			testkit.WriteFile(t, jar, cases[name])
		}
		var got []string
		for _, op := range []string{"extract", "fill", "fidelity"} {
			root := filepath.Join(dir, name, "go-"+op)
			// An absent root is part of the result: Go must not create
			// one where Python did not.
			got = append(append(got, op+":", goDocJars(op, install, root), "tree:"), treeOrAbsent(t, root)...)
		}
		// Fidelity over what extract left: an existing file makes it read
		// the entry, so the reader's errors surface here too.
		got = append(got, "fidelity after extract:", goDocJars("fidelity", install, filepath.Join(dir, name, "go-extract")))
		testkit.CheckFrozen(t, "docjar-corners/"+name, got)
	}
}

// treeOrAbsent is treeLines, or one "<absent>" line when root does not
// exist.
func treeOrAbsent(t *testing.T, root string) []string {
	t.Helper()
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return []string{"<absent>"}
	}
	return testkit.TreeLines(t, root)
}

func keysOfBytes(m map[string][]byte) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func goDocJars(op, install, root string) string {
	var v pyjson.Value
	var err error
	switch op {
	case "extract":
		var s *ExtractStats
		s, err = Extract(install, root, nil)
		v = extractValue(s)
	case "fill":
		var w []string
		w, err = FillMissing(install, root, nil)
		v = pyjson.NewObject()
		v.Set("written", stringsValue(w))
	case "fidelity":
		var r *FidelityReport
		r, err = Fidelity(install, root, nil)
		v = fidelityValue(r)
	}
	if err != nil {
		return `["raise"]`
	}
	return testkit.DumpLine(testkit.Str("ok"), v)
}

// mapPairs are the synthetic jars' central-directory names, plus a grid of
// jar kinds against entry shapes.
func mapPairs(t *testing.T, jars []string) [][2]any {
	t.Helper()
	var pairs [][2]any
	for _, j := range jars {
		infos, err := zipfile.ReadCentralDir(j)
		if err != nil {
			continue
		}
		names := []string{}
		for _, i := range infos {
			names = append(names, i.Name)
		}
		pairs = append(pairs, [2]any{filepath.Base(j), names})
	}
	kinds := []string{JavadocJar, "docDeveloper-doc.jar", "docDeveloper-ux.jar", "docDeveloper-rt.jar",
		"docSource-doc.jar", "docJdk-doc.jar", "docHoneywell.jar", "clHVAC-doc.jar", "doc.jar", "doc-doc.jar",
		"docX-rt.jar", "docdoc.jar", "Doc-doc.jar", "docDeveloperAnalytics-doc.jar", "x-DOC.jar", "docDeveloper.jar",
		"docSource.jar", "docJdk.jar", "bajaScript-ux.jar", "workbench-wb.jar", "galileoSupervisor-wb.jar",
		"micros-wb.jar", "micros-ux.jar", "batchJob-ux.jar", "other.jar", "ascCommon.jar", "ñdoc-doc.jar"}
	entries := []string{"doc/", "doc/a.html", "doc/x/y.bajadoc", "rc/a.md", "rc/x/B.java", "rc/x/B.class",
		"META-INF/MANIFEST.MF", "META-INF/", "examples/a.js", "a.class", "help.html", "res/alertonHome.html",
		"jsdoc/a.html", "jsdoc/scripts/URI.js", "com/tridium/workbench/media/about.html", "doc/style.css",
		"doc/STYLE.CSS", "doc/lib-1.0.tar.gz", "doc/lib.TGZ", "", "doc", "rc/", "docx/a", "doc/rc/a.md",
		"rc/env/x.md", "rc/node_modules/a/README.md", "rc/ReadMe.txt", "rc/html/altHelp.html", "doc/a.html",
		"rc/a.md", "x/doc/a.html", "doc/ñ/ü.html"}
	for _, k := range kinds {
		pairs = append(pairs, [2]any{k, entries})
	}
	return pairs
}

// goMap is, per jar of pairs, its kind tests and per entry its destination
// and help test, then the mapped entries with their collisions.
func goMap(t *testing.T, pairs [][2]any) []string {
	rules := mustDefault(t)
	var out []string
	for _, p := range pairs {
		jar, names := p[0].(string), p[1].([]string)
		out = append(out, testkit.DumpLine(testkit.Str("jar"), testkit.Str(jar), testkit.Boolean(IsDocJar(jar)), testkit.Boolean(IsModuleJar(jar)),
			testkit.Str(JarStem(jar)), testkit.Str(GuideFolder(jar))))
		for _, e := range names {
			rel, ok := DestPath(jar, e, rules)
			d := pyjson.Value{}
			if ok {
				d = testkit.Str(rel)
			}
			out = append(out, testkit.DumpLine(testkit.Str("entry"), testkit.Str(e), d, testkit.Boolean(IsHelpEntry(jar, e, rules))))
		}
		for _, m := range MappedEntries(jar, names, rules) {
			first := pyjson.Value{}
			if m.Collides {
				first = testkit.Str(m.First)
			}
			out = append(out, testkit.DumpLine(testkit.Str("mapped"), testkit.IntV(m.Pos), testkit.Str(m.Entry), testkit.Str(m.Rel), first))
		}
	}
	return out
}

// TestDocJarMappingFrozen: the doc-jar mapping of the synthetic install and
// corner jars, and the help and doc jars of the install.
func TestDocJarMappingFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	install := syntheticInstall(t, dir)
	jars, _ := filepath.Glob(filepath.Join(install, "modules", "*.jar"))
	jars = append(jars, syntheticJars(t, filepath.Join(dir, "corners"))...)
	lines := testkit.Relabeled(append(goMap(t, mapPairs(t, jars)), goHelpJars(t, install)...), install, "<install>")
	testkit.CheckFrozen(t, "extract/mapping", perJar(lines))
}

// perJar appends each "entry" and "mapped" line to the line before it, so
// a frozen file holds one line per jar.
func perJar(lines []string) []string {
	var out []string
	for _, l := range lines {
		if len(out) > 0 && (strings.HasPrefix(l, `["entry", `) || strings.HasPrefix(l, `["mapped", `)) {
			out[len(out)-1] += " " + l
			continue
		}
		out = append(out, l)
	}
	return out
}

func goHelpJars(t *testing.T, install string) []string {
	t.Helper()
	help, err1 := HelpJars(install, nil, mustDefault(t))
	docs, err2 := ListDocJars(install, true, nil)
	if err1 != nil || err2 != nil {
		t.Fatalf("help jars: %v %v", err1, err2)
	}
	return []string{testkit.DumpLine(testkit.Str("help"), stringsValue(help)), testkit.DumpLine(testkit.Str("doc"), stringsValue(docs))}
}

// TestEmptyIndexesFrozen: _empty_indexes on a root holding some indexes:
// the lines and the tree. Intended difference (decision I3-5): Python also
// wrote an empty call-index.json and its line; the Go output, frozen here,
// is Python's without them.
func TestEmptyIndexesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	root := filepath.Join(t.TempDir(), "go")
	testkit.WriteFile(t, filepath.Join(root, "indexes", "class-index.json"), []byte(`{"x": 1}`))
	testkit.WriteFile(t, filepath.Join(root, "indexes", "pdf-index.json"), []byte("{}\n"))
	lines, err := EmptyIndexes(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "indexes", "call-index.json")); !os.IsNotExist(err) {
		t.Errorf("EmptyIndexes wrote call-index.json (retired, I3-5): %v", err)
	}
	testkit.CheckFrozen(t, "extract/empty-indexes", append([]string{pyjson.Dumps(stringsValue(lines), pyfs.DumpDefault), "tree:"},
		testkit.TreeLines(t, root)...))
}
