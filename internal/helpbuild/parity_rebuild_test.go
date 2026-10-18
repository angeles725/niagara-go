package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyref"
)

// Parity of rebuild-all (S10a) through testdata/rebuild/driver.py. Each
// scenario runs on one data-root path: built, run by Python (its builders
// as child processes), observed, removed, built again, run by Go,
// observed. The same path on both sides keeps every printed path and the
// directory order the unsorted walks follow equal, so the stdout and
// stderr text and the whole tree (build.log included) are compared as
// they are, after the normalizations of normalizeRebuild and
// rebuildTreeLines (run times, stamped_utc, a builder's traceback, the
// intended PDF page count).

// rebuildParityVar lists the version ids TestParityRebuildReal rebuilds.
const rebuildParityVar = "HELPBUILD_PARITY_REBUILD"

// runRebuildDriver runs testdata/rebuild/driver.py with the CPython 3.15
// of driver() on the tools/ of the pinned niagara-help reference
// (pyref.Dir: the pre-go-retire worktree), with its Python escape set.
func runRebuildDriver(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "rebuild", "driver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	cmd.Env = append(os.Environ(), pyref.Env("niagara-help")...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("rebuild driver %v: %v\n%s", args, err, errb.String())
	}
	return out.String(), errb.String()
}

// goRebuildStep is the Go side of one driver step on root.
func goRebuildStep(t *testing.T, root string, step []string) (stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	switch step[0] {
	case "run":
		opt := RebuildOptions{Stdout: &out, Stderr: &errb}
		for i := 1; i < len(step); i++ {
			switch step[i] {
			case "--dry-run":
				opt.DryRun = true
			case "--stamp-only":
				opt.StampOnly = true
			case "--only":
				i++
				opt.Only = step[i]
			}
		}
		if _, err := RebuildAll(root, opt); err != nil {
			t.Fatalf("RebuildAll %v: %v", step, err)
		}
	case "log":
		tail, err := rebuild(root, RebuildToLog(&errb))
		if err != nil {
			t.Fatalf("rebuild: %v", err)
		}
		for _, l := range tail {
			out.WriteString(l + "\n")
		}
	default:
		t.Fatalf("unknown step %v", step)
	}
	return out.String(), errb.String()
}

var (
	rebuildRunTime = regexp.MustCompile(`(Done in |Time: +)\d+\.\d( ?s)`)
	rebuildStamped = regexp.MustCompile(`(Manifest stamped at )\S+( \(baseline)`)
	stampedField   = regexp.MustCompile(`"stamped_utc": "([^"]*)"`)
	isoMicros      = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{6})?\+00:00$`)
)

// normalizeRebuild splits one stream into lines with the run times and
// the stamp time replaced; on stderr, a Python traceback (the header to
// the exception line) and Go's "ERROR: <script> raised: ..." line both
// become "<raised>" (intended difference: the text of a crash).
func normalizeRebuild(text string, stderr bool) []string {
	text = rebuildRunTime.ReplaceAllString(text, "${1}<t>${2}")
	text = rebuildStamped.ReplaceAllString(text, "${1}<utc>${2}")
	var out []string
	inTrace := false
	for _, l := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		case !stderr:
		case l == "Traceback (most recent call last):":
			inTrace = true
			continue
		case inTrace && (strings.HasPrefix(l, " ") || l == ""):
			continue
		case inTrace:
			inTrace = false
			l = "<raised>"
		case strings.HasPrefix(l, "ERROR: ") && strings.Contains(l, " raised: "):
			l = "<raised>"
		}
		out = append(out, l)
	}
	return out
}

// rebuildTreeLines is treeLines of root with indexes/.manifest.json as
// its text, stamped_utc normalized, and build.log as its text, run times
// normalized (both set aside while treeLines walks, then put back); on the Python side, once pdf_extractor has run in the
// scenario, the PDF page counts are adjusted as pyPDFTreeLines does
// (intended difference; a pdf-index.json copied in is compared as it is).
func rebuildTreeLines(t *testing.T, root string, python, pdfRan bool) []string {
	t.Helper()
	if !pycompat.IsDir(root) {
		return []string{"<absent>"}
	}
	path := filepath.Join(root, "indexes", ".manifest.json")
	var manifest []string
	if data, perm, restore, ok := takeAside(t, path); ok {
		m := stampedField.FindSubmatch(data)
		if m == nil || !isoMicros.Match(m[1]) {
			t.Errorf("%s: stamped_utc missing or not isoformat()", path)
		}
		manifest = append([]string{fmt.Sprintf("indexes/.manifest.json %o", perm)},
			strings.Split(stampedField.ReplaceAllString(string(data), `"stamped_utc": "<utc>"`), "\n")...)
		defer restore()
	}
	// build.log records each builder's "Done in N.Ns": compared as text with
	// the run times normalized, not hashed (a hash differed whenever the two
	// sides' timings differed in one digit).
	logPath := filepath.Join(root, "build.log")
	var buildLog []string
	if data, perm, restore, ok := takeAside(t, logPath); ok {
		buildLog = append([]string{fmt.Sprintf("build.log %o", perm)},
			normalizeRebuild(string(data), false)...)
		defer restore()
	}
	var lines []string
	if python && pdfRan {
		lines = pyPDFTreeLines(t, root, root)
	} else {
		lines = treeLines(t, root)
	}
	return append(append(lines, manifest...), buildLog...)
}

// takeAside reads the file at path and removes it until restore puts it
// back with the same bytes and permission bits (a Chmod after the write:
// the create mode alone is lowered by the umask). ok is false when there
// is no file; anything that is not a regular file (a symlink, a folder)
// fails the test, since putting it back as a file would change the tree.
func takeAside(t *testing.T, path string) (data []byte, perm os.FileMode, restore func(), ok bool) {
	t.Helper()
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %v, not a regular file: it cannot be set aside and restored", path, info.Mode().Type())
	}
	if data, err = os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	perm = info.Mode().Perm()
	return data, perm, func() {
		if err := os.WriteFile(path, data, perm); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, perm); err != nil {
			t.Fatal(err)
		}
	}, true
}

// rebuildBoth runs steps on a fresh setup(root) on each side and compares
// the observations; it returns both run times (steps only).
func rebuildBoth(t *testing.T, what, root string, setup func(root string), steps ...[]string) (py, gol time.Duration) {
	t.Helper()
	side := func(python bool) (obs []string, took time.Duration) {
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		setup(root)
		pdfRan := false
		for _, step := range steps {
			pdfRan = pdfRan || runsPDF(step)
			start := time.Now()
			var out, errs string
			if python {
				out, errs = runRebuildDriver(t, append([]string{step[0], root}, step[1:]...)...)
			} else {
				out, errs = goRebuildStep(t, root, step)
			}
			took += time.Since(start)
			obs = append(obs, "## "+strings.Join(step, " "), "# stdout")
			obs = append(obs, normalizeRebuild(out, false)...)
			obs = append(obs, "# stderr")
			obs = append(obs, normalizeRebuild(errs, true)...)
			obs = append(obs, "# tree")
			obs = append(obs, rebuildTreeLines(t, root, python, pdfRan)...)
		}
		return obs, took
	}
	pyObs, py := side(true)
	goObs, gol := side(false)
	diffLines(t, what, pyObs, goObs)
	return py, gol
}

// runsPDF reports whether a driver step runs pdf_extractor.py: the hook,
// or a run that is neither a dry run nor --stamp-only and whose --only
// keeps it.
func runsPDF(step []string) bool {
	if step[0] == "log" {
		return true
	}
	for i := 1; i < len(step); i++ {
		switch step[i] {
		case "--dry-run", "--stamp-only":
			return false
		case "--only":
			if i+1 < len(step) && !strings.HasPrefix("pdf_extractor.py", step[i+1]) {
				return false
			}
		}
	}
	return true
}

// TestParityRebuildPlan: Go's plan is maintenance.BUILDERS without
// build_call_index.py (the one row taken out, call-index.json being
// retired), skip-if always None, and the same OPTIONAL_ARTIFACTS.
func TestParityRebuildPlan(t *testing.T) {
	out, _ := runRebuildDriver(t, "plan")
	var py struct {
		Removed  []string          `json:"removed"`
		Builders [][]any           `json:"builders"`
		Optional map[string]string `json:"optional"`
	}
	if err := json.Unmarshal([]byte(out), &py); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if strings.Join(py.Removed, " ") != "build_call_index.py" {
		t.Errorf("taken out of BUILDERS: %q, want build_call_index.py only", py.Removed)
	}
	var gol [][]any
	for _, b := range Builders() {
		produces := make([]any, len(b.Produces))
		for i, p := range b.Produces {
			produces[i] = p
		}
		var src any
		if b.Source != "" {
			src = b.Source
		}
		gol = append(gol, []any{b.Script, produces, src, nil})
	}
	g, err := json.Marshal(gol)
	if err != nil {
		t.Fatal(err)
	}
	p, err := json.Marshal(py.Builders)
	if err != nil {
		t.Fatal(err)
	}
	if string(g) != string(p) {
		t.Errorf("plan:\n go     %s\n python %s", g, p)
	}
	if fmt.Sprint(py.Optional) != fmt.Sprint(OptionalArtifacts) {
		t.Errorf("OPTIONAL_ARTIFACTS: python %v, go %v", py.Optional, OptionalArtifacts)
	}
}

// rebuildFixture is a data root every builder has input for: the S7a, S7b,
// S8 and S9 fixtures merged, plus the S5 guide pages.
func rebuildFixture(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{"s8", "s9", "bajadoc", "bajadocxml"} {
		copyTree(t, filepath.Join("testdata", d, "root"), root)
	}
	guidesFixture(t, root)
}

// TestParityRebuildSynthetic: the plan, --only, a full run, --stamp-only,
// a second run and the build.log hook on the merged fixture; a root with
// no corpus folder; a run with a builder that exits 1 and one that raises
// after a stamp (the manifest is kept); --only matching nothing.
func TestParityRebuildSynthetic(t *testing.T) {
	driver(t)
	if runtime.GOOS == "windows" {
		t.Skip("parity runs on Linux")
	}
	root := filepath.Join(t.TempDir(), "root")
	rebuildBoth(t, "fixture", root, func(r string) { rebuildFixture(t, r) },
		[]string{"run", "--dry-run"}, []string{"run", "--dry-run", "--only", "build_"},
		[]string{"run"}, []string{"run", "--stamp-only"}, []string{"run"}, []string{"log"})
	rebuildBoth(t, "empty root", root, func(r string) { writeFile(t, filepath.Join(r, "readme.txt"), nil) },
		[]string{"run", "--dry-run"}, []string{"run"}, []string{"log"})
	rebuildBoth(t, "failing builders", root, func(r string) {
		writeFile(t, filepath.Join(r, "bajadoc", "x.html"), nil) // no bajadoc/javax/baja: exit 1
		writeFile(t, filepath.Join(r, "source", "m-rt", "a", "A.java"), []byte("package a;\nimport b.B;\npublic class A {}\n"))
		writeFile(t, filepath.Join(r, "source", "m-rt", "b", "B.java"), []byte("package b;\nimport c.;\npublic class B {}\n"))
		writeFile(t, filepath.Join(r, "docs", "notes.md"), []byte("not a manual"))
	}, []string{"run", "--stamp-only"}, []string{"run"}, []string{"log"})
	rebuildBoth(t, "only nothing", root, func(r string) { rebuildFixture(t, r) }, []string{"run", "--only", "zzz"})
}

// rebuildInputs are the data-root entries TestParityRebuildReal copies:
// the raw and derived folders, the PDF text, the course text and the old
// build.log (the 4.14 data root is the niagara-help folder itself).
var rebuildInputs = append(append(append([]string(nil), RawDirs...), DerivedDirs...), "docs", "docs-text", "course-text", "build.log")

// TestParityRebuildReal rebuilds copies of real version roots on both
// sides (HELPBUILD_PARITY_REBUILD=<id>[:<id>...]), each registered in a
// temporary repo with its real install_dir and course_dir, so the PDF
// manuals and the course are rebuilt too; it logs both run times. The
// version roots are untracked (versions/*/ is ignored), so they come from
// the live niagara-help checkout (niagaraHelpData), the Python code from
// the pinned reference (runRebuildDriver).
func TestParityRebuildReal(t *testing.T) {
	if !parityOn(t, rebuildParityVar) {
		t.Skipf("real-data parity runs only on request: set %s=<version id>[:<id>...]", rebuildParityVar)
	}
	driver(t)
	var vids []string
	for _, v := range filepath.SplitList(os.Getenv(rebuildParityVar)) {
		if v != "" {
			vids = append(vids, v)
		}
	}
	if len(vids) == 0 {
		t.Fatalf("%s names no version id", rebuildParityVar)
	}
	help := niagaraHelpData()
	data, err := os.ReadFile(filepath.Join(help, "versions", "registry.json"))
	if err != nil {
		missingReference(t, fmt.Sprintf("no registry: %v", err))
	}
	var reg struct {
		Versions map[string]map[string]string `json:"versions"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			entry, ok := reg.Versions[vid]
			if !ok {
				t.Fatalf("%s: version %s is not registered", rebuildParityVar, vid)
			}
			src := filepath.Join(help, filepath.FromSlash(entry["data_root"]))
			if !pycompat.IsDir(src) {
				missingReference(t, fmt.Sprintf("version %s has no data root (%s)", vid, src))
			}
			repo := t.TempDir()
			copied := map[string]string{"label": entry["label"], "install_dir": entry["install_dir"], "data_root": "versions/" + vid}
			if c := entry["course_dir"]; c != "" {
				copied["course_dir"] = filepath.Join(help, filepath.FromSlash(c))
			}
			if !pycompat.IsDir(entry["install_dir"]) {
				t.Logf("%s: install %s not mounted (the PDF step reports it on both sides)", vid, entry["install_dir"])
			}
			regData, err := json.Marshal(map[string]any{"default": vid, "versions": map[string]any{vid: copied}})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(repo, "versions", "registry.json"), regData)
			root := filepath.Join(repo, "versions", vid)
			setup := func(root string) {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
				for _, d := range rebuildInputs {
					if _, err := os.Lstat(filepath.Join(src, d)); err != nil {
						continue
					}
					if out, err := exec.Command("cp", "-a", filepath.Join(src, d), filepath.Join(root, d)).CombinedOutput(); err != nil {
						t.Fatalf("copy %s: %v\n%s", d, err, out)
					}
				}
			}
			py, gol := rebuildBoth(t, vid, root, setup, []string{"run", "--dry-run"}, []string{"run"})
			t.Logf("%s rebuild-all: python %.1fs, go %.1fs", vid, py.Seconds(), gol.Seconds())
		})
	}
}
