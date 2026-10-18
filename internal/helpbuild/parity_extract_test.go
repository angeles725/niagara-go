package helpbuild

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// S2 parity: doc-jar mapping, extraction, fill-missing, fidelity and the
// version manifest, Python (driver.py map / helpjars / docjars / setup)
// against this package, on synthetic installs and a 4.3 subset. Trees are
// compared in full: every path, kind, permission bits, size and sha256
// (manifests with generated_utc normalized, inline).

func intV(n int) pyjson.Value { return pyjson.IntValue(int64(n)) }

func collisionsValue(cs []Collision) pyjson.Value {
	a := make([]pyjson.Value, len(cs))
	for i, c := range cs {
		a[i] = array(str(c.Rel), str(c.Owner), str(c.By))
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

// extractValue is doc_jars.extract's dict, null for nil.
func extractValue(s *ExtractStats) pyjson.Value {
	if s == nil {
		return pyjson.Value{}
	}
	v := pyjson.NewObject()
	v.Set("files", intV(s.Files))
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
	v.Set("checked", intV(r.Checked))
	v.Set("matched", intV(r.Matched))
	v.Set("mismatched", stringsValue(r.Mismatched))
	v.Set("missing", stringsValue(r.Missing))
	v.Set("eol_only", stringsValue(r.EOLOnly))
	v.Set("collisions", collisionsValue(r.Collisions))
	v.Set("jars", intV(r.Jars))
	return v
}

func dumpLine(vs ...pyjson.Value) string { return pyjson.Dumps(array(vs...), DumpDefault) }

// pyRaise turns Python's ["raise", "<Class>"] into ["raise"]: Go has no
// exception classes, the decision (an error, not a result) is compared.
func pyRaise(t *testing.T, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, `["raise", `) {
			t.Logf("python raised %s", l)
			l = `["raise"]`
		}
		out[i] = l
	}
	return out
}

// goSetup is driver.py setup rebuilt from ExtractAndIndex.
func goSetup(repo, vid, op string) []string {
	res, err := ExtractAndIndex(repo, vid, SetupOptions{Force: op == "force", VerifyOnly: op == "verify", FillMissing: op == "fill"})
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, dumpLine(str("out"), str(l)))
		}
	}
	if err != nil {
		return append(out, `["raise"]`)
	}
	return append(out, dumpLine(str("result"), intV(res.Status), optStr(res.Error),
		extractValue(res.Extracted), fidelityValue(res.Fidelity)))
}

// setupStep runs one extract_and_index on both repos and compares the
// result lines and both whole trees.
func setupStep(t *testing.T, pyRepo, goRepo, vid, op string) {
	t.Helper()
	what := vid + " " + op
	// The data root is the one path that differs: it is named <repo>.
	py := relabel(pyRaise(t, runDriver(t, "", "setup", pyRepo, vid, op)), pyRepo)
	diffLines(t, what+" result", py, relabel(goSetup(goRepo, vid, op), goRepo))
	diffLines(t, what+" tree", treeLines(t, pyRepo), treeLines(t, goRepo))
}

func relabel(lines []string, repo string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.ReplaceAll(l, repo, "<repo>")
	}
	return out
}

// onBoth applies one tree mutation to the Python and the Go data root.
func onBoth(t *testing.T, roots []string, f func(root string)) {
	t.Helper()
	for _, r := range roots {
		f(r)
	}
}

func TestParityExtractSynthetic(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	install := syntheticInstall(t, dir)
	pyRepo, goRepo := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	for _, repo := range []string{pyRepo, goRepo} {
		writeRegistry(t, repo, map[string][3]string{
			"1.0.0.0": {"fixture ñ", install, "versions/1.0.0.0"},
			"1.0.0.1": {"repo root", install, "."},
			"9.9.9.9": {"not mounted", install + "-missing", "versions/9.9.9.9"},
		})
	}
	setupStep(t, pyRepo, goRepo, "9.9.9.9", "extract") // install dir not found
	setupStep(t, pyRepo, goRepo, "1.0.0.1", "extract") // refuses the repo root
	setupStep(t, pyRepo, goRepo, "1.0.0.1", "verify")  // allowed there: all missing
	setupStep(t, pyRepo, goRepo, "1.0.0.0", "extract")
	setupStep(t, pyRepo, goRepo, "1.0.0.0", "extract") // already holds: refused
	roots := []string{filepath.Join(pyRepo, "versions", "1.0.0.0"), filepath.Join(goRepo, "versions", "1.0.0.0")}
	onBoth(t, roots, func(r string) {
		writeFile(t, filepath.Join(r, "guides", "ClHVACTest", "Tasks.html"), []byte("tampered"))
		writeFile(t, filepath.Join(r, "guides", "ClHVACTest", "crlf.txt"), []byte("a\nb\n"))
		writeFile(t, filepath.Join(r, "guides", "Stored", "s.txt"), []byte("stored\nline\n"))
		writeFile(t, filepath.Join(r, "devguide-rc", "com", "x", "A.java"), []byte("class A {}\n"))
		for _, gone := range []string{"guides/ClHVACTest/README.txt", "devguide-rc/com/x/BVal.java", "bajadoc/index.html"} {
			if err := os.Remove(filepath.Join(r, filepath.FromSlash(gone))); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, filepath.Join(r, "guides", "stray.txt"), []byte("removed by --force"))
		writeFile(t, filepath.Join(r, "indexes", "old.json"), []byte("{}"))
		writeFile(t, filepath.Join(r, "keep.txt"), []byte("outside the raw and derived dirs"))
	})
	setupStep(t, pyRepo, goRepo, "1.0.0.0", "verify") // mismatched, eol-only, missing
	setupStep(t, pyRepo, goRepo, "1.0.0.0", "fill")   // writes only the missing
	setupStep(t, pyRepo, goRepo, "1.0.0.0", "force")  // wipes raw + derived dirs
}

// The doc-jar library calls on installs that make Python raise or take a
// corner: entry-reader errors, unreadable doc jars, a jar that is a folder.
func TestParityDocJarCorners(t *testing.T) {
	driver(t)
	ok := rawEntry{name: "doc/ok.html", data: "ok"}
	cases := map[string][]byte{
		"badcrc":    rawZip("", []rawEntry{ok, {name: "doc/a.txt", data: "abc", crcXor: 1}}, ""),
		"localname": rawZip("", []rawEntry{{name: "doc/a.txt", data: "abc", localName: "doc/b.txt"}}, ""),
		"encrypted": rawZip("", []rawEntry{{name: "doc/a.txt", data: "abc", flags: 1}}, ""),
		"method99":  rawZip("", []rawEntry{{name: "doc/a.txt", data: "abc", method: 99}}, ""),
		"overlap":   rawZip("", []rawEntry{{name: "doc/a.txt", data: "abc", compPlus: 4}, ok}, ""),
		"shared":    rawZip("", []rawEntry{{name: "doc/a.txt", data: "abc"}, {name: "doc/a.txt", data: "abc", sharedHeader: 1}}, ""),
		"short":     rawZip("", []rawEntry{{name: "doc/a.txt", data: "hello", shortSize: 3}}, ""),
		"cp437":     rawZip("", []rawEntry{{name: "doc/\x81ber.html", data: "u"}, {name: "doc/n.html\x00.exe", data: "n"}}, ""),
		"prepended": rawZip(strings.Repeat("#", 33), []rawEntry{ok, {name: "doc/b.txt", data: "b"}}, ""),
		"zip64":     zip64Zip("", []rawEntry{ok}, 1, 0),
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
			writeFile(t, jar, cases[name])
		}
		for _, op := range []string{"extract", "fill", "fidelity"} {
			pyRoot, goRoot := filepath.Join(dir, name, "py-"+op), filepath.Join(dir, name, "go-"+op)
			py := pyRaise(t, runDriver(t, "", "docjars", op, install, pyRoot))
			diffLines(t, name+" "+op, py, []string{goDocJars(op, install, goRoot)})
			// An absent root is part of the result: Go must not create
			// one where Python did not.
			diffLines(t, name+" "+op+" tree", treeOrAbsent(t, pyRoot), treeOrAbsent(t, goRoot))
		}
		// Fidelity over what extract left: an existing file makes it read
		// the entry, so the reader's errors surface here too.
		pyRoot, goRoot := filepath.Join(dir, name, "py-extract"), filepath.Join(dir, name, "go-extract")
		py := pyRaise(t, runDriver(t, "", "docjars", "fidelity", install, pyRoot))
		diffLines(t, name+" fidelity after extract", py, []string{goDocJars("fidelity", install, goRoot)})
	}
}

// treeOrAbsent is treeLines, or one "<absent>" line when root does not
// exist.
func treeOrAbsent(t *testing.T, root string) []string {
	t.Helper()
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return []string{"<absent>"}
	}
	return treeLines(t, root)
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
	return dumpLine(str("ok"), v)
}

// mapPairs is driver.py map's input: the synthetic jars' central-directory
// names, plus a grid of jar kinds against entry shapes.
func mapPairs(t *testing.T, jars []string) [][2]any {
	t.Helper()
	var pairs [][2]any
	for _, j := range jars {
		infos, err := readCentralDir(j)
		if err != nil {
			continue
		}
		names := []string{}
		for _, i := range infos {
			names = append(names, i.name)
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

// goMap is driver.py map rebuilt from this package.
func goMap(t *testing.T, pairs [][2]any) []string {
	rules := mustDefault(t)
	var out []string
	for _, p := range pairs {
		jar, names := p[0].(string), p[1].([]string)
		out = append(out, dumpLine(str("jar"), str(jar), boolean(IsDocJar(jar)), boolean(IsModuleJar(jar)),
			str(JarStem(jar)), str(GuideFolder(jar))))
		for _, e := range names {
			rel, ok := DestPath(jar, e, rules)
			d := pyjson.Value{}
			if ok {
				d = str(rel)
			}
			out = append(out, dumpLine(str("entry"), str(e), d, boolean(IsHelpEntry(jar, e, rules))))
		}
		for _, m := range MappedEntries(jar, names, rules) {
			first := pyjson.Value{}
			if m.Collides {
				first = str(m.First)
			}
			out = append(out, dumpLine(str("mapped"), intV(m.Pos), str(m.Entry), str(m.Rel), first))
		}
	}
	return out
}

func TestParityDocJarMapping(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	install := syntheticInstall(t, dir)
	jars, _ := filepath.Glob(filepath.Join(install, "modules", "*.jar"))
	jars = append(jars, syntheticJars(t, filepath.Join(dir, "corners"))...)
	pairs := mapPairs(t, jars)
	in, err := json.Marshal(pairs)
	if err != nil {
		t.Fatal(err)
	}
	diffLines(t, "map", runDriver(t, string(in), "map"), goMap(t, pairs))
	diffLines(t, "help jars", runDriver(t, "", "helpjars", install), goHelpJars(t, install))
}

func goHelpJars(t *testing.T, install string) []string {
	t.Helper()
	help, err1 := HelpJars(install, nil, mustDefault(t))
	docs, err2 := ListDocJars(install, true, nil)
	if err1 != nil || err2 != nil {
		t.Fatalf("help jars: %v %v", err1, err2)
	}
	return []string{dumpLine(str("help"), stringsValue(help)), dumpLine(str("doc"), stringsValue(docs))}
}

// realSubset: five doc jars of the EC-Net 4.3 install (the biggest, the
// docDeveloper example-code pair, a small and a mid-size guide) plus two
// module jars with extract-rule pages and one without docs, copied into a
// fake install (never written in place).
var realSubset = []string{"docDeveloper-doc.jar", "docDeveloper-ux.jar", "docDeveloper-rt.jar",
	"docLinuxN4-doc.jar", "docObix-doc.jar", "workbench-wb.jar", "batchJob-ux.jar", "alarm-rt.jar"}

const install43 = "/mnt/c/Niagara/EC-Net4-4.3.58.18"

func TestParityExtractReal(t *testing.T) {
	needRealParity(t)
	driver(t)
	if _, err := os.Stat(filepath.Join(install43, "modules")); err != nil {
		t.Skipf("4.3 install not mounted (%v): parity not checked", err)
	}
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	for _, name := range realSubset {
		copyFile(t, filepath.Join(install43, "modules", name), filepath.Join(install, "modules", name))
	}
	pyRepo, goRepo := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	for _, repo := range []string{pyRepo, goRepo} {
		writeRegistry(t, repo, map[string][3]string{"4.3.58.18": {"EC-Net 4.3 subset", install, "versions/4.3.58.18"}})
	}
	setupStep(t, pyRepo, goRepo, "4.3.58.18", "extract")
	setupStep(t, pyRepo, goRepo, "4.3.58.18", "verify")
	n, size := 0, int64(0)
	_ = filepath.WalkDir(filepath.Join(goRepo, "versions", "4.3.58.18"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if st, err := d.Info(); err == nil {
				n, size = n+1, size+st.Size()
			}
		}
		return nil
	})
	t.Logf("4.3 subset: %d extracted files, %d bytes compared", n, size)
}

// The whole 4.3 install: help_jars lists every module jar (~25 s per side
// over /mnt/c), so it runs only with HELPBUILD_PARITY_FULL=1.
func TestParityHelpJarsFullInstall(t *testing.T) {
	if os.Getenv("HELPBUILD_PARITY_FULL") == "" {
		t.Skip("HELPBUILD_PARITY_FULL unset: the whole-install help_jars sweep is not run")
	}
	driver(t)
	diffLines(t, "4.3 help jars", runDriver(t, "", "helpjars", install43), goHelpJars(t, install43))
}

// _empty_indexes on a root holding some indexes: Python and Go write the
// same {} files and the same lines.
func TestParityEmptyIndexes(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	var roots []string
	for _, side := range []string{"py", "go"} {
		root := filepath.Join(dir, side)
		writeFile(t, filepath.Join(root, "indexes", "class-index.json"), []byte(`{"x": 1}`))
		writeFile(t, filepath.Join(root, "indexes", "pdf-index.json"), []byte("{}\n"))
		roots = append(roots, root)
	}
	py := runDriver(t, "", "emptyindexes", roots[0])
	lines, err := EmptyIndexes(roots[1])
	if err != nil {
		t.Fatal(err)
	}
	diffLines(t, "_empty_indexes", py, []string{pyjson.Dumps(stringsValue(lines), DumpDefault)})
	diffLines(t, "_empty_indexes tree", treeLines(t, roots[0]), treeLines(t, roots[1]))
}
