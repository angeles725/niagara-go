package helpbuild

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// `versions add` (version_setup.add_version and version_report) against
// its frozen Python output (frozen_test.go). Each scenario runs in one
// repository path under $HOME (never /tmp, as the Python run did); per
// step, stdout and the exit status, then how the observed repository
// changed (lineDelta). The observation is the whole repository tree with
// these fields normalized (they hold the time of the run or the mtimes of
// the files it extracted):
//   - generated_utc of versions/<id>.manifest.json (isoformat, seconds)
//   - stamped_utc of <data root>/indexes/.manifest.json
//   - the "Done in"/"Time:" run times in <data root>/build.log
//   - in <data root>/indexes/fulltext.sqlite (listed as its docs and meta
//     rows, not its bytes), the newest-mtime and SHA-1 fields of each
//     source of the meta fingerprint
//
// The frozen files were checked against the retired testdata/versions/driver.py, with
// Python's PDF page counts adjusted as Go counts them (intended difference
// of S7a). stderr (the builders' output and open_index's notes) is not
// recorded.

// homeTemp is a temporary folder under $HOME, removed at the end of t.
func homeTemp(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(home, ".helpbuild-versions-frozen-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// goAdd runs AddVersion with the command-line arguments args (repo,
// install, flags).
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
func addTreeLines(t *testing.T, repo, vid string) []string {
	t.Helper()
	if !pycompat.IsDir(repo) {
		return []string{"<absent>"}
	}
	root := filepath.Join(repo, "versions", vid)
	aside := repo + ".aside"
	var lines []string
	observeRoot := func() { lines = append(lines, rebuildTreeLines(t, root)...) }
	if ft := filepath.Join(root, "indexes", "fulltext.sqlite"); pycompat.IsFile(ft) {
		lines = append(lines, "fulltext.sqlite:")
		lines = append(lines, fulltextRows(t, ft)...)
		inner := observeRoot
		observeRoot = func() { setAside(t, ft, aside+".sqlite", inner) }
	}
	if bl := filepath.Join(root, "build.log"); pycompat.IsFile(bl) {
		lines = append(lines, "build.log:")
		lines = append(lines, normalizeRebuild(testkit.ReadText(t, bl), false)...)
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
		setAside(t, skip, aside, func() { lines = testkit.TreeLines(t, repo) })
		return lines
	}
	return testkit.TreeLines(t, repo)
}

// pathFiles are the files (relative to the data root or the repository)
// whose bytes hold the repository path: a frozen observation lists their
// text, dir relabeled, not their hash.
var pathFiles = map[string]bool{"gates.log": true, "versions/registry.json": true}

// addObservation is addTreeLines with the repository-path files of
// pathFiles shown as text and the listing-cache entries masked.
func addObservation(t *testing.T, dir, repo, vid string) []string {
	t.Helper()
	var out []string
	base := repo // addTreeLines lists the data root, then the repository
	for _, l := range maskListingCache(addTreeLines(t, repo, vid)) {
		switch l {
		case "data root:":
			base = filepath.Join(repo, "versions", vid)
		case "repository:":
			base = repo
		}
		f := strings.Fields(l)
		if len(f) != 5 || !pathFiles[f[0]] {
			out = append(out, l)
			continue
		}
		out = append(out, strings.Join(f[:3], " ")+" text:")
		for _, line := range strings.Split(testkit.ReadText(t, filepath.Join(base, filepath.FromSlash(f[0]))), "\n") {
			out = append(out, "| "+strings.ReplaceAll(line, dir, "<dir>"))
		}
	}
	return out
}

// addFrozen runs steps (each the arguments after the repository) on a
// fresh setup(repo) and checks, per step, its stdout, exit status and the
// observation delta against testdata/frozen/versions/<name>.txt, the
// temporary folder dir shown as <dir>.
func addFrozen(t *testing.T, name, dir, repo, vid string, setup func(repo string), steps ...[]string) {
	t.Helper()
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	setup(repo)
	prev := addObservation(t, dir, repo, vid)
	var obs []string
	for _, step := range steps {
		out, code := goAdd(t, append([]string{repo}, step...)...)
		obs = append(append(obs, "## "+strings.Join(step, " "), "# stdout"), strings.Split(out, "\n")...)
		cur := addObservation(t, dir, repo, vid)
		obs = append(append(obs, fmt.Sprintf("# exit %d", code), "# tree"), testkit.LineDelta(prev, cur)...)
		prev = cur
	}
	testkit.CheckFrozen(t, "versions/"+name, testkit.Relabeled(obs, dir, "<dir>"))
}

// TestVersionsAddFrozen: on the synthetic install (every doc-jar
// mapping, an unreadable jar) with a baja.jar: the refusals before
// registering, a full add, the same add again (refused), a forced re-add
// with --default and --label, and a data root that already holds a raw
// folder (refused, registry rolled back).
func TestVersionsAddFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := homeTemp(t)
	install := syntheticInstall(t, dir)
	writeJar(t, filepath.Join(install, "modules", "baja.jar"), [2]string{"META-INF/module.xml", moduleXML("1.0.0.0")})
	repo := filepath.Join(dir, "repo")
	noop := func(r string) {
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	addFrozen(t, "refusals", dir, repo, "1.0.0.0", noop, []string{filepath.Join(dir, "nope")},
		[]string{install, "--id", "4.15"}, []string{install, "--id", ""}, []string{install, "--id", "١.٢.٣.٤x"})
	addFrozen(t, "add", dir, repo, "1.0.0.0", noop, []string{install}, []string{install},
		[]string{install, "--force", "--default", "--label", "Synthetic ñ"}, []string{install + "/.", "--id", "1.0.0.0"})
	addFrozen(t, "existing-data-root", dir, repo, "1.0.0.0", func(r string) {
		testkit.WriteFile(t, filepath.Join(r, "versions", "1.0.0.0", "guides", "keep.html"), []byte("kept"))
		writeRegistry(t, r, map[string][3]string{"9.9.9.9": {"Other", "/elsewhere", "versions/9.9.9.9"}})
	}, []string{install})
}
