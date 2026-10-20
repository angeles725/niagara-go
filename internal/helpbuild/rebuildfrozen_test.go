package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// rebuild-all (S10a) against its frozen Python output (frozen_test.go):
// per scenario and step, stdout and stderr after normalizeRebuild (run
// times, the stamp time, a builder's traceback), the tree delta and the
// manifest and build.log texts after rebuildTreeLines (stamped_utc and run
// times normalized). The frozen files were checked against the retired
// testdata/rebuild/driver.py, which ran Python's builders as child
// processes on the same data-root path.

// goRebuildStep runs one step on root: "run" with its rebuild-all flags,
// or "log" (the build.log hook of versions add).
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
// normalized (both set aside while treeLines walks, then put back).
func rebuildTreeLines(t *testing.T, root string) []string {
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
	return append(append(testkit.TreeLines(t, root), manifest...), buildLog...)
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

// rebuildFrozen runs steps on a fresh setup(root) and checks, per step,
// its streams, the tree delta and the manifest and build.log texts against
// testdata/frozen/rebuild/<name>.txt.
func rebuildFrozen(t *testing.T, name, root string, setup func(root string), steps ...[]string) {
	t.Helper()
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	setup(root)
	prev, _ := splitRebuildTree(rebuildTreeLines(t, root))
	var obs []string
	for _, step := range steps {
		out, errs := goRebuildStep(t, root, step)
		obs = append(obs, "## "+strings.Join(step, " "), "# stdout")
		obs = append(obs, normalizeRebuild(out, false)...)
		obs = append(obs, "# stderr")
		obs = append(obs, normalizeRebuild(errs, true)...)
		cur, texts := splitRebuildTree(rebuildTreeLines(t, root))
		obs = append(append(obs, "# tree"), testkit.TreeDelta(prev, cur)...)
		obs = append(append(obs, "# texts"), texts...)
		prev = cur
	}
	testkit.CheckFrozen(t, "rebuild/"+name, testkit.Relabeled(obs, root, "<root>"))
}

// splitRebuildTree splits rebuildTreeLines output into the tree entries and
// the manifest and build.log texts after them (each starts at its
// "<name> <perm>" header line).
func splitRebuildTree(lines []string) (tree, texts []string) {
	for i, l := range lines {
		if f := strings.Fields(l); len(f) == 2 && (f[0] == "indexes/.manifest.json" || f[0] == "build.log") {
			return lines[:i], lines[i:]
		}
	}
	return lines, nil
}

// TestRebuildPlanFrozen: Go's plan is maintenance.BUILDERS without
// build_call_index.py (the one row taken out, call-index.json being
// retired), skip-if always None, and the same OPTIONAL_ARTIFACTS.
func TestRebuildPlanFrozen(t *testing.T) {
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
	testkit.CheckFrozen(t, "rebuild/plan", []string{string(g), fmt.Sprint(OptionalArtifacts)})
}

// rebuildFixture is a data root every builder has input for: the S7a, S7b,
// S8 and S9 fixtures (indexes/testdata) merged, plus the S5 guide pages.
func rebuildFixture(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{"s8", "s9", "bajadoc", "bajadocxml"} {
		testkit.CopyTree(t, filepath.Join("indexes", "testdata", d, "root"), root)
	}
	guidesFixture(t, root)
}

// TestRebuildFrozen: the plan, --only, a full run, --stamp-only, a second
// run and the build.log hook on the merged fixture; a root with no corpus
// folder; a run with a builder that exits 1 and one that raises after a
// stamp (the manifest is kept); --only matching nothing.
func TestRebuildFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	root := filepath.Join(t.TempDir(), "root")
	rebuildFrozen(t, "fixture", root, func(r string) { rebuildFixture(t, r) },
		[]string{"run", "--dry-run"}, []string{"run", "--dry-run", "--only", "build_"},
		[]string{"run"}, []string{"run", "--stamp-only"}, []string{"run"}, []string{"log"})
	rebuildFrozen(t, "empty-root", root, func(r string) { testkit.WriteFile(t, filepath.Join(r, "readme.txt"), nil) },
		[]string{"run", "--dry-run"}, []string{"run"}, []string{"log"})
	rebuildFrozen(t, "failing-builders", root, func(r string) {
		testkit.WriteFile(t, filepath.Join(r, "bajadoc", "x.html"), nil) // no bajadoc/javax/baja: exit 1
		testkit.WriteFile(t, filepath.Join(r, "source", "m-rt", "a", "A.java"), []byte("package a;\nimport b.B;\npublic class A {}\n"))
		testkit.WriteFile(t, filepath.Join(r, "source", "m-rt", "b", "B.java"), []byte("package b;\nimport c.;\npublic class B {}\n"))
		testkit.WriteFile(t, filepath.Join(r, "docs", "notes.md"), []byte("not a manual"))
	}, []string{"run", "--stamp-only"}, []string{"run"}, []string{"log"})
	rebuildFrozen(t, "only-nothing", root, func(r string) { rebuildFixture(t, r) }, []string{"run", "--only", "zzz"})
}
