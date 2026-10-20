package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// The coverage command runs the helpbuild reports in process
// (cli/cmd_export.py run_coverage). These tests drive nh end to end on a
// synthetic checkout and check that it prints exactly what the helpbuild
// entry points print and exits as run_coverage does; the bytes of the
// reports themselves are pinned against frozen Python output by helpbuild's
// covfrozen_test.go.

// maintRepo writes a checkout whose version 1.0 has its data root at the
// repository root and its install at install: four report indexes with
// two modules, one documented class, and (gap) a guide folder with an
// HTML page that has no extracted text.
func maintRepo(t *testing.T, install string, gap bool) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "versions", "registry.json"),
		`{"default": "1.0", "versions": {"1.0": {"label": "t", "data_root": ".", "install_dir": `+
			quoteJSON(install)+`}}}`)
	files := map[string]string{
		"indexes/class-index.json":       `{"BAlarm": {"package": "javax.baja.alarm"}, "BZeta": {}}`,
		"indexes/source-index.json":      `{"BAlarm": {"module": "alarm-rt"}, "BZeta": {"module": "zeta-rt"}}`,
		"indexes/method-index.json":      `{"ack": [{"class": "BAlarm", "section": "methods"}]}`,
		"indexes/slots-index.json":       `{"BAlarm": {}}`,
		"bajadoc-clean/alarm/BAlarm.txt": "x",
		"guides/Ok/a.html":               "a",
		"guides-clean/Ok/a.txt":          "a",
	}
	if gap {
		files["guides/Spy/b.html"] = "b"
	}
	for rel, body := range files {
		write(t, filepath.Join(dir, filepath.FromSlash(rel)), body)
	}
	return dir
}

func quoteJSON(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// emptyInstall is an install folder with no jar and no docs/.
func emptyInstall(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "install")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// helpbuildCoverage is what run_coverage prints and returns for the plain
// report, computed with the library alone.
func helpbuildCoverage(t *testing.T, repo, module string) (string, int) {
	t.Helper()
	var b strings.Builder
	res, err := helpbuild.Coverage(&b, repo, module)
	if err != nil {
		t.Fatal(err)
	}
	if res.Miss {
		lines, err := scope.Miss(repo, "coverage", module, "modules", nil)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(lines, "\n") + "\n", 1
	}
	return b.String(), min(res.GuideGaps, 1)
}

func TestCoverageReport(t *testing.T) {
	for _, tt := range []struct {
		name   string
		gap    bool
		args   []string
		module string
		code   int
	}{
		{name: "overview", args: []string{"coverage"}, code: 0},
		{name: "overview with a guide gap exits 1", gap: true, args: []string{"coverage"}, code: 1},
		{name: "module", args: []string{"coverage", "alarm-rt"}, module: "alarm-rt", code: 0},
		{name: "module ignores guide gaps", gap: true, args: []string{"coverage", "alarm"}, module: "alarm", code: 0},
		{name: "module miss", args: []string{"coverage", "nosuch"}, module: "nosuch", code: 1},
		{name: "empty module is the overview", args: []string{"coverage", ""}, code: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := maintRepo(t, emptyInstall(t), tt.gap)
			out, errs, code := run(t, repo, tt.args...)
			want, wantCode := helpbuildCoverage(t, repo, tt.module)
			if out != want || errs != "" || code != wantCode || code != tt.code {
				t.Errorf("code %d (library %d, want %d) stderr %q\nstdout:\n%s\nwant:\n%s",
					code, wantCode, tt.code, errs, out, want)
			}
		})
	}
}

func TestCoverageMissingIndex(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	if err := os.Remove(filepath.Join(repo, "indexes", "slots-index.json")); err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, repo, "coverage")
	want := "ERROR: " + (&helpbuild.MissingIndexError{Name: "slots-index.json", BaseDir: repo}).Error() + "\n"
	if code != 2 || out != "" || errs != want {
		t.Errorf("code %d stdout %q stderr %q; want 2, %q", code, out, errs, want)
	}
}

// gateStatus runs the doc-jar gate or the sweep with the library alone on
// repo (version 1.0), with nh's rebuilding full-text reader.
func gateStatus(t *testing.T, repo string, sweep bool) (string, int) {
	t.Helper()
	var b bytes.Buffer
	e := &Env{Out: &b, Err: &b, RepoRoot: repo, BaseDir: repo, VersionID: "1.0"}
	var status int
	var err error
	if sweep {
		status, err = helpbuild.RunSweep(repo, repo, "1.0", &b, helpbuild.SweepOptions{Fulltext: e.fulltextDocs})
	} else {
		status, err = helpbuild.DocJarCoverage(&b, repo, repo, "1.0", e.fulltextDocs)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.String(), status
}

func TestCoverageGates(t *testing.T) {
	missing := func(t *testing.T) string { return filepath.Join(t.TempDir(), "not-mounted") }
	for _, tt := range []struct {
		name    string
		install func(t *testing.T) string
		gap     bool
		args    []string
		docJars bool // the doc-jar report comes first
		sweep   bool // the sweep report follows
		code    int
	}{
		{name: "doc-jars pass", install: emptyInstall, args: []string{"coverage", "--doc-jars"}, docJars: true, code: 0},
		{name: "doc-jars fail on a guide gap", install: emptyInstall, gap: true,
			args: []string{"coverage", "--doc-jars"}, docJars: true, code: 1},
		{name: "doc-jars install not available", install: missing,
			args: []string{"coverage", "--doc-jars"}, docJars: true, code: 2},
		{name: "doc-jars ignores the module", install: emptyInstall,
			args: []string{"coverage", "alarm-rt", "--doc-jars"}, docJars: true, code: 0},
		{name: "sweep pass", install: emptyInstall, args: []string{"coverage", "--sweep"}, sweep: true, code: 0},
		{name: "sweep install not available", install: missing,
			args: []string{"coverage", "--sweep", "nosuch"}, sweep: true, code: 2},
		{name: "sweep and doc-jars", install: emptyInstall,
			args: []string{"coverage", "--sweep", "--doc-jars"}, docJars: true, sweep: true, code: 0},
		{name: "sweep and failing doc-jars exit with the worst", install: emptyInstall, gap: true,
			args: []string{"coverage", "--doc-jars", "--sweep"}, docJars: true, sweep: true, code: 1},
		{name: "unavailable doc-jars stop before the sweep", install: missing,
			args: []string{"coverage", "--sweep", "--doc-jars"}, docJars: true, code: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := maintRepo(t, tt.install(t), tt.gap)
			out, errs, code := run(t, repo, tt.args...)
			want, wantCode := "", 0
			if tt.docJars {
				report, status := gateStatus(t, repo, false)
				want, wantCode = report, status
				if strings.Contains(strings.Join(tt.args, " "), "--sweep") {
					want += "\n"
				}
			}
			if tt.sweep {
				report, status := gateStatus(t, repo, true)
				want, wantCode = want+report, max(wantCode, status)
			}
			if out != want || errs != "" || code != wantCode || code != tt.code {
				t.Errorf("code %d (library %d, want %d) stderr %q\nstdout:\n%s\nwant:\n%s",
					code, wantCode, tt.code, errs, out, want)
			}
		})
	}
}

// TestCoverageFulltextDocsRebuilds: the gates read the full-text index
// through nh's opener, which builds a missing index (with its notice on
// stderr and under its lock) where helpbuild's default reader fails.
func TestCoverageFulltextDocsRebuilds(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	write(t, filepath.Join(repo, "guides-clean", "Ok", "page.txt"), "alarm routing\n")
	if _, err := helpbuild.FreshFulltextDocs(repo); err == nil {
		t.Fatal("the fixture already has an up-to-date full-text index")
	}
	var out, errs bytes.Buffer
	e := &Env{Out: &out, Err: &errs, RepoRoot: repo, BaseDir: repo, VersionID: "1.0"}
	docs, err := e.fulltextDocs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 || !strings.Contains(errs.String(), "building the full-text index for "+repo) {
		t.Errorf("docs %v, stderr %q", docs, errs.String())
	}
	if _, err := os.Stat(fts.DBPath(repo)); err != nil {
		t.Errorf("the index was not built in place: %v", err)
	}
	errs.Reset()
	again, err := e.fulltextDocs(repo)
	if err != nil || len(again) != len(docs) || errs.Len() != 0 {
		t.Errorf("second read: %d docs (want %d), err %v, stderr %q", len(again), len(docs), err, errs.String())
	}
}

func TestCoverageHelp(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	out, errs, code := run(t, repo, "coverage", "--help")
	for _, want := range []string{"usage: niagara_help coverage [-h] [--doc-jars] [--sweep] [module]",
		"Module name (omit for overview)", "--doc-jars", "--sweep"} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage --help lacks %q:\n%s", want, out)
		}
	}
	if code != 0 || errs != "" {
		t.Errorf("code %d stderr %q", code, errs)
	}
	top, _, _ := run(t, repo, "--help")
	if !strings.Contains(top, "    coverage            Documentation coverage report") {
		t.Errorf("--help does not list coverage:\n%s", top)
	}
}
