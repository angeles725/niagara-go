package helpbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyref"
)

// Parity of `versions add` (version_setup.add_version and
// version_report) through testdata/versions/driver.py. Each scenario runs
// in one repository path under $HOME (never /tmp): set up, run by Python
// (its builders as child processes), observed, removed, set up again, run
// by Go, observed. The same paths on both sides keep every printed path
// equal, so stdout and the exit status are compared as they are and the
// whole repository tree byte for byte, except these fields (they hold the
// time of the run or the mtimes of the files it extracted):
//   - generated_utc of versions/<id>.manifest.json (isoformat, seconds)
//   - stamped_utc of <data root>/indexes/.manifest.json
//   - the "Done in"/"Time:" run times in <data root>/build.log
//   - in <data root>/indexes/fulltext.sqlite (compared as its docs and
//     meta rows, not its bytes: the two SQLite builders write different
//     pages), the newest-mtime and SHA-1 fields of each source of the meta
//     fingerprint
//
// and Python's PDF page counts in pdf-index.json, adjusted as
// pyPDFTreeLines does (intended difference of S7a). stderr (the builders'
// output and open_index's notes) is not compared.

// versionsAddParityVar names a real install TestParityVersionsAddReal
// copies a subset of (baja.jar, the realSubset jars it has, its two
// smallest PDF manuals).
const versionsAddParityVar = "HELPBUILD_PARITY_VERSIONS_ADD"

func init() { parityVars = append(parityVars, versionsAddParityVar) }

// homeTemp is a temporary folder under $HOME, removed at the end of t.
func homeTemp(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(home, ".helpbuild-versions-parity-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// runVersionsDriver runs driver.py add with the pinned reference.
func runVersionsDriver(t *testing.T, args ...string) (stdout string, code int) {
	t.Helper()
	python, _ := driver(t)
	tools := filepath.Join(pyref.Dir(t, "niagara-help"), "tools")
	script, err := filepath.Abs(filepath.Join("testdata", "versions", "driver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools, "add"}, args...)...)
	cmd.Env = append(os.Environ(), pyref.Env("niagara-help")...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() <= 2 && !strings.Contains(errb.String(), "Traceback") {
		return out.String(), ee.ExitCode()
	} else if err != nil {
		t.Fatalf("versions driver %v: %v\n%s", args, err, errb.String())
	}
	return out.String(), 0
}

// goAdd is the Go side of one driver call.
func goAdd(t *testing.T, args ...string) (stdout string, code int) {
	t.Helper()
	repo, install := args[0], args[1]
	opt := AddOptions{Rebuild: RebuildToLog(nil), Fulltext: buildingDocs}
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--label":
			i++
			opt.Label = args[i]
		case "--id":
			i++
			opt.ID = &args[i]
		case "--default":
			opt.Default = true
		case "--force":
			opt.Force = true
		}
	}
	var out bytes.Buffer
	code, err := AddVersion(repo, install, &out, opt)
	if err != nil {
		t.Fatalf("AddVersion %v: %v\n%s", args, err, out.String())
	}
	return out.String(), code
}

var ftsFingerprintTimes = regexp.MustCompile(`:\d+:[0-9a-f]{40}\b`)

// fulltextRows is the docs and meta tables of a full-text database, the
// fingerprint's mtime fields normalized.
func fulltextRows(t *testing.T, path string) []string {
	t.Helper()
	db, err := index.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var out []string
	for _, q := range []string{`SELECT 'meta', key, value, '', '', '' FROM meta ORDER BY key`,
		`SELECT 'docs', id, source, path, title, line || ' ' || page FROM docs ORDER BY id`} {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var c [6]any
			if err := rows.Scan(&c[0], &c[1], &c[2], &c[3], &c[4], &c[5]); err != nil {
				t.Fatal(err)
			}
			out = append(out, ftsFingerprintTimes.ReplaceAllString(fmt.Sprintf("%v", c), ":<mtime>:<sha1>"))
		}
		rows.Close()
	}
	return out
}

// setAside moves path out of the tree while f runs.
func setAside(t *testing.T, path, aside string, f func()) {
	t.Helper()
	if err := os.Rename(path, aside); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(aside, path); err != nil {
			t.Fatal(err)
		}
	}()
	f()
}

// addTreeLines is the observed repository: rebuildTreeLines of the data
// root without build.log and fulltext.sqlite (both listed apart,
// normalized), then every other path of the repository (treeLines'
// format, the version manifest with generated_utc normalized).
func addTreeLines(t *testing.T, repo, vid string, python bool) []string {
	t.Helper()
	if !pycompat.IsDir(repo) {
		return []string{"<absent>"}
	}
	root := filepath.Join(repo, "versions", vid)
	aside := repo + ".aside"
	var lines []string
	observeRoot := func() { lines = append(lines, pyRebuildTreeLines(t, root, python)...) }
	if ft := filepath.Join(root, "indexes", "fulltext.sqlite"); pycompat.IsFile(ft) {
		lines = append(lines, "fulltext.sqlite:")
		lines = append(lines, fulltextRows(t, ft)...)
		inner := observeRoot
		observeRoot = func() { setAside(t, ft, aside+".sqlite", inner) }
	}
	if bl := filepath.Join(root, "build.log"); pycompat.IsFile(bl) {
		lines = append(lines, "build.log:")
		lines = append(lines, normalizeRebuild(readText(t, bl), false)...)
		inner := observeRoot
		observeRoot = func() { setAside(t, bl, aside+".log", inner) }
	}
	if pycompat.IsDir(root) {
		lines = append(lines, "data root:")
		observeRoot()
	}
	lines = append(lines, "repository:")
	return append(lines, treeLinesExcept(t, repo, root)...)
}

// treeLinesExcept is treeLines of repo without the subtree skip (set
// aside while treeLines walks).
func treeLinesExcept(t *testing.T, repo, skip string) []string {
	t.Helper()
	aside := repo + ".skipped"
	var lines []string
	if pycompat.IsDir(skip) {
		setAside(t, skip, aside, func() { lines = treeLines(t, repo) })
		return lines
	}
	return treeLines(t, repo)
}

// addStep is one `versions add` call observed on one side.
func addStep(t *testing.T, python bool, repo, vid string, args ...string) []string {
	t.Helper()
	var out string
	var code int
	if python {
		out, code = runVersionsDriver(t, append([]string{repo}, args...)...)
	} else {
		out, code = goAdd(t, append([]string{repo}, args...)...)
	}
	obs := append([]string{"## " + strings.Join(args, " "), "# stdout"}, strings.Split(out, "\n")...)
	obs = append(obs, fmt.Sprintf("# exit %d", code), "# tree")
	return append(obs, addTreeLines(t, repo, vid, python)...)
}

// addBoth runs the steps on a fresh setup(repo) on each side and compares
// the observations; it returns both run times.
func addBoth(t *testing.T, what, repo, vid string, setup func(repo string), steps ...[]string) (py, gol time.Duration) {
	t.Helper()
	side := func(python bool) (obs []string, took time.Duration) {
		if err := os.RemoveAll(repo); err != nil {
			t.Fatal(err)
		}
		setup(repo)
		start := time.Now()
		for _, step := range steps {
			obs = append(obs, addStep(t, python, repo, vid, step...)...)
		}
		return obs, time.Since(start)
	}
	pyObs, py := side(true)
	goObs, gol := side(false)
	diffLines(t, what, pyObs, goObs)
	return py, gol
}

// TestParityVersionsAddSynthetic: on the synthetic install (every doc-jar
// mapping, an unreadable jar) with a baja.jar: the refusals before
// registering, a full add, the same add again (refused), a forced re-add
// with --default and --label, and a data root that already holds a raw
// folder (refused, registry rolled back).
func TestParityVersionsAddSynthetic(t *testing.T) {
	driver(t)
	if runtime.GOOS == "windows" {
		t.Skip("parity runs on Linux")
	}
	dir := homeTemp(t)
	install := syntheticInstall(t, dir)
	writeJar(t, filepath.Join(install, "modules", "baja.jar"), [2]string{"META-INF/module.xml", moduleXML("1.0.0.0")})
	repo := filepath.Join(dir, "repo")
	noop := func(r string) {
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	addBoth(t, "refusals", repo, "1.0.0.0", noop, []string{filepath.Join(dir, "nope")},
		[]string{install, "--id", "4.15"}, []string{install, "--id", ""}, []string{install, "--id", "١.٢.٣.٤x"})
	addBoth(t, "add", repo, "1.0.0.0", noop, []string{install}, []string{install},
		[]string{install, "--force", "--default", "--label", "Synthetic ñ"}, []string{install + "/.", "--id", "1.0.0.0"})
	addBoth(t, "existing data root", repo, "1.0.0.0", func(r string) {
		writeFile(t, filepath.Join(r, "versions", "1.0.0.0", "guides", "keep.html"), []byte("kept"))
		writeRegistry(t, r, map[string][3]string{"9.9.9.9": {"Other", "/elsewhere", "versions/9.9.9.9"}})
	}, []string{install})
}

// TestParityVersionsAddReal adds a subset copy of a real install
// (HELPBUILD_PARITY_VERSIONS_ADD=<install>): its baja.jar (so the id is
// detected), the realSubset jars it has and its two smallest PDF manuals,
// copied under $HOME; it logs both run times.
func TestParityVersionsAddReal(t *testing.T) {
	src := os.Getenv(versionsAddParityVar)
	if src == "" {
		t.Skipf("real-data parity runs only on request: set %s=<install>", versionsAddParityVar)
	}
	driver(t)
	if !pycompat.IsFile(filepath.Join(src, "modules", "baja.jar")) {
		t.Fatalf("%s=%s: no modules/baja.jar", versionsAddParityVar, src)
	}
	dir := homeTemp(t)
	install := filepath.Join(dir, filepath.Base(src))
	for _, name := range append([]string{"baja.jar"}, realSubset...) {
		if from := filepath.Join(src, "modules", name); pycompat.IsFile(from) {
			copyFile(t, from, filepath.Join(install, "modules", name))
		}
	}
	for _, pdf := range smallestPDFs(t, filepath.Join(src, "docs"), 2) {
		copyFile(t, filepath.Join(src, "docs", pdf), filepath.Join(install, "docs", pdf))
	}
	vid, err := DetectVersion(install)
	if err != nil || !ValidVersionID(vid) {
		t.Fatalf("version of %s: %q %v", src, vid, err)
	}
	repo := filepath.Join(dir, "repo")
	py, gol := addBoth(t, vid, repo, vid, func(r string) {
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
	}, []string{install})
	t.Logf("%s subset versions add: python %.1fs, go %.1fs", vid, py.Seconds(), gol.Seconds())
}

// smallestPDFs are the n smallest *.pdf files under docs (relative paths;
// ties by path).
func smallestPDFs(t *testing.T, docs string, n int) []string {
	t.Helper()
	type pdf struct {
		rel  string
		size int64
	}
	var all []pdf
	_ = filepath.Walk(docs, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(p), ".pdf") {
			rel, _ := filepath.Rel(docs, p)
			all = append(all, pdf{rel, info.Size()})
		}
		return nil
	})
	sort.Slice(all, func(i, j int) bool {
		if all[i].size != all[j].size {
			return all[i].size < all[j].size
		}
		return all[i].rel < all[j].rel
	})
	var out []string
	for i := 0; i < n && i < len(all); i++ {
		out = append(out, all[i].rel)
	}
	return out
}

// realSubset: five doc jars of the EC-Net 4.3 install (the biggest, the
// docDeveloper example-code pair, a small and a mid-size guide) plus two
// module jars with extract-rule pages and one without docs, copied into a
// fake install (never written in place).
var realSubset = []string{"docDeveloper-doc.jar", "docDeveloper-ux.jar", "docDeveloper-rt.jar",
	"docLinuxN4-doc.jar", "docObix-doc.jar", "workbench-wb.jar", "batchJob-ux.jar", "alarm-rt.jar"}

// The whole 4.3 install: help_jars lists every module jar (~25 s per side
// over /mnt/c), so it runs only with HELPBUILD_PARITY_FULL=1.

// pyPDFTreeLines is treeLines of the Python tree root with the intended
// page-count difference taken out: in base's pdf-index.json every entry
// whose docs-text/<stem>.txt ends with a form feed has one page less
// (Python counts that trailing form feed as a page, Go does not). The file
// on disk is left as Python wrote it, since the next step reads it back.
func pyPDFTreeLines(t *testing.T, root, base string) []string {
	t.Helper()
	lines := treeLines(t, root)
	path := filepath.Join(base, "indexes", "pdf-index.json")
	v, err := loadJSON(path)
	if err != nil || v.Kind != pyjson.Object {
		return lines // missing or not an index: compared as it is
	}
	var less []string // the stems whose pages Python counted one too many
	for _, stem := range v.Keys {
		p, ok := v.M[stem].Get("pages")
		if !ok || p.Kind != pyjson.Int || p.I.Sign() <= 0 {
			continue
		}
		text, err := os.ReadFile(filepath.Join(base, "docs-text", stem+".txt"))
		if err != nil || !bytes.HasSuffix(text, []byte("\f")) {
			continue
		}
		less = append(less, stem)
	}
	if len(less) == 0 {
		return lines // byte for byte: also an index the test wrote itself
	}
	// The adjusted index is re-dumped by Go, so first require that a re-dump
	// of Python's index as it is reproduces Python's bytes: a serialization
	// drift must fail here, not hide in the rebuilt hash.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diffLines(t, "pyjson.Dumps of Python's pdf-index.json", strings.Split(string(raw), "\n"),
		strings.Split(pyjson.Dumps(v, DumpPDFIndex), "\n"))
	for _, stem := range less {
		e := v.M[stem]
		p, _ := e.Get("pages")
		e.Set("pages", pyjson.IntValue(p.I.Int64()-1))
	}
	data := []byte(pyjson.Dumps(v, DumpPDFIndex))
	sum := sha256.Sum256(data)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range lines {
		if f := strings.Fields(l); len(f) == 5 && f[0] == filepath.ToSlash(rel) {
			lines[i] = fmt.Sprintf("%s %s %s %d %s", f[0], f[1], f[2], len(data), hex.EncodeToString(sum[:]))
		}
	}
	return lines
}

// pyRebuildTreeLines is rebuildTreeLines, with Python's pdf-index.json
// line adjusted as pyPDFTreeLines does on the Python side.
func pyRebuildTreeLines(t *testing.T, root string, python bool) []string {
	t.Helper()
	lines := rebuildTreeLines(t, root)
	if !python || !pycompat.IsDir(root) {
		return lines
	}
	for _, rel := range []string{filepath.Join("indexes", ".manifest.json"), "build.log"} {
		if _, _, restore, ok := takeAside(t, filepath.Join(root, rel)); ok {
			defer restore()
		}
	}
	adjusted := map[string]string{}
	for _, l := range pyPDFTreeLines(t, root, root) {
		if f := strings.Fields(l); len(f) == 5 && strings.HasSuffix(f[0], "pdf-index.json") {
			adjusted[f[0]] = l
		}
	}
	for i, l := range lines {
		if f := strings.Fields(l); len(f) == 5 && adjusted[f[0]] != "" {
			lines[i] = adjusted[f[0]]
		}
	}
	return lines
}
