package helpbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// S4 parity: course.plan and course_extractor.main()
// (testdata/parity/textdriver.py plan / course) against CoursePlan and
// CourseExtractor, on a synthetic course and on the real
// sources/n4-dev-course. Compared: every plan row with the "(course)"
// exclude rule, stdout, stderr and the exit status, and both repository
// trees byte for byte.

// courseExtras widen courseFixture over the course globs: text below
// text/, an upper-case extension, dev/ readme.md, .js and doc pages,
// slide doc pages, PDFs outside course/, key material, build files,
// non-ASCII names, a file that matches nothing.
var courseExtras = map[string]string{
	"text/sub/nested.txt":                               "nested page\n",
	"text/UPPER.TXT":                                    "upper\n",
	"text/notes.md":                                     "not a page\n",
	"niagara-4.15.6.20/dev/demo/README.md":              "# Demo\n",
	"niagara-4.15.6.20/dev/demo/readme-first.txt":       "first\n",
	"niagara-4.15.6.20/dev/demo/src/rc/app.js":          "define([], {});\n",
	"niagara-4.15.6.20/dev/demo/src/doc/index.html":     "<html>demo</html>\n",
	"niagara-4.15.6.20/dev/demo/build.gradle.kts":       "plugins {}\n",
	"course/Slide Code/slides/slides-ux/src/doc/p.html": "<p>slide</p>\n",
	"course/Labs/Lab2 Ñandú/src/BÑandú.java":            "class BÑandú {}\n",
	"course/Labs/Lab5 Code Signing/server.key":          "KEY",
	"course/Labs/Lab5 Code Signing/certs/ca.p12":        "P12",
	"solutions/lab2/BSolved.java":                       "class BSolved {}\n",
	"solutions/readme.pdf":                              "%PDF-1.4",
	"Course Overview.pdf":                               "%PDF-1.4",
	"course/Labs/Lab1 First/build.xml":                  "<project/>\n",
	"course/Labs/Lab1 First/src/doc/lab.html":           "<html/>\n",
	"misc/unmatched.bin":                                "?",
}

func syntheticCourse(t *testing.T, src string) {
	writeTree(t, src, courseFixture)
	writeTree(t, src, courseExtras)
	if err := os.Symlink(filepath.Join(src, "solutions"), filepath.Join(src, "course", "linked-solutions")); err != nil {
		t.Fatal(err)
	}
}

// goPlanLines is textdriver.py plan rebuilt from CoursePlan.
func goPlanLines(t *testing.T, src string) []string {
	rules := mustDefault(t)
	var out []string
	for _, r := range CoursePlan(src, rules) {
		rule := ""
		if x := rules.ExcludeRule(CoursePseudoJar, r.Rel); x != nil {
			rule = x.ID
		}
		out = append(out, dumpLine(str(r.Rel), optStr(r.Kind), optStr(r.Dest), optStr(rule)))
	}
	return out
}

func coursePlanParity(t *testing.T, what, src string) {
	t.Helper()
	py := runDriverLines(t, "plan", src)
	gol := goPlanLines(t, src)
	diffLines(t, what+" plan", py, gol)
	kinds := map[string]int{}
	for _, r := range CoursePlan(src, mustDefault(t)) {
		kinds[r.Kind]++
	}
	t.Logf("%s: %d files, kinds %v", what, len(gol), kinds)
}

// runDriverLines is runTextDriver's raw JSON lines.
func runDriverLines(t *testing.T, args ...string) []string {
	var out []string
	for _, row := range runTextDriver(t, args...) {
		vs := make([]pyjson.Value, len(row))
		for i, x := range row {
			switch x := x.(type) {
			case string:
				vs[i] = str(x)
			case nil:
				vs[i] = pyjson.Value{}
			default:
				t.Fatalf("unexpected plan value %v", x)
			}
		}
		out = append(out, dumpLine(vs...))
	}
	return out
}

// courseStep runs course_extractor on data root rel of both repositories
// and compares the streams, the status and both repository trees.
func courseStep(t *testing.T, what, pyRepo, goRepo, rel string) {
	t.Helper()
	py := pyToolLines(t, pyRepo, runTextDriver(t, "course", filepath.Join(pyRepo, rel)))
	out, err := CourseExtractor(filepath.Join(goRepo, rel), mustDefault(t))
	outcome := fmt.Sprintf("exit %d", out.Status)
	if err != nil {
		t.Logf("go error: %v", err)
		outcome = "raise"
	}
	join := func(ls []string) string {
		if len(ls) == 0 {
			return ""
		}
		return strings.Join(ls, "\n") + "\n"
	}
	gol := toolLines(goRepo, map[string]string{"stdout": join(out.Stdout), "stderr": join(out.Stderr)},
		[]string{"stdout", "stderr"}, outcome)
	diffLines(t, what+" streams", py, gol)
	diffLines(t, what+" tree", treeLines(t, pyRepo), treeLines(t, goRepo))
}

func TestParityCoursePlanSynthetic(t *testing.T) {
	driver(t)
	src := filepath.Join(t.TempDir(), "course-src")
	syntheticCourse(t, src)
	coursePlanParity(t, "synthetic course", src)
}

func TestParityCourseExtractorSynthetic(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "course-src")
	syntheticCourse(t, src)
	pyRepo, goRepo := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	repos := []string{pyRepo, goRepo}
	for _, repo := range repos {
		writeCourseRegistry(t, repo, map[string]string{
			"A": "../course-src", "B": "", "C": "../no-course", "D": src,
		})
	}
	courseStep(t, "bound by a relative course_dir", pyRepo, goRepo, "versions/A")
	onBoth(t, repos, func(r string) {
		writeFile(t, filepath.Join(r, "versions", "B", "course-text", "stale.txt"), []byte("earlier binding"))
		writeFile(t, filepath.Join(r, "versions", "B", "devguide-examples", "course", "x", "Stale.java"), []byte("x"))
		writeFile(t, filepath.Join(r, "versions", "B", "devguide-examples", "other.txt"), []byte("not ours"))
	})
	courseStep(t, "unbound: earlier copies removed", pyRepo, goRepo, "versions/B")
	onBoth(t, repos, func(r string) {
		b := filepath.Join(r, "versions", "B")
		writeFile(t, filepath.Join(b, "elsewhere", "x.txt"), []byte("behind a symlink"))
		if err := os.Symlink(filepath.Join(b, "elsewhere"), filepath.Join(b, "course-text")); err != nil {
			t.Fatal(err)
		}
	})
	courseStep(t, "unbound: rmtree refuses a symlink", pyRepo, goRepo, "versions/B")
	courseStep(t, "bound course_dir missing", pyRepo, goRepo, "versions/C")
	courseStep(t, "bound by an absolute course_dir", pyRepo, goRepo, "versions/D")
	if err := os.Remove(filepath.Join(src, "text", "8 ORDs.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "text", "9 New.txt"), []byte("new page\n"))
	courseStep(t, "rebuilt after the course changed", pyRepo, goRepo, "versions/A")
	courseStep(t, "unregistered root", pyRepo, goRepo, "versions")
	if os.Geteuid() != 0 {
		locked := filepath.Join(src, "course", "Labs", "Lab1 First", "src", "BFirst.java")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		courseStep(t, "a failed copy keeps the previous copies", pyRepo, goRepo, "versions/A")
		if err := os.Chmod(locked, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if mkfifo, err := exec.LookPath("mkfifo"); err == nil {
		fifo := filepath.Join(src, "text", "pipe.txt")
		if out, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
			t.Fatalf("mkfifo: %v %s", err, out)
		}
		courseStep(t, "a named pipe is refused, never opened", pyRepo, goRepo, "versions/A")
		if err := os.Remove(fifo); err != nil {
			t.Fatal(err)
		}
	}
	onBoth(t, repos, func(r string) {
		writeFile(t, filepath.Join(r, "versions", "registry.json"), []byte("{"))
	})
	courseStep(t, "registry not JSON", pyRepo, goRepo, "versions/A")
}

// writeCourseRegistry registers one version per id (data root
// versions/<id>, created) with its course_dir ("" for none).
func writeCourseRegistry(t *testing.T, repo string, courseDirs map[string]string) {
	vers := pyjson.NewObject()
	for _, id := range pyjson.SortedKeys(keysOfStr(courseDirs)) {
		e := pyjson.NewObject()
		e.Set("label", pyjson.String(id))
		e.Set("install_dir", pyjson.String("/nonexistent"))
		e.Set("data_root", pyjson.String("versions/"+id))
		if courseDirs[id] != "" {
			e.Set("course_dir", pyjson.String(courseDirs[id]))
		}
		vers.Set(id, e)
		if err := os.MkdirAll(filepath.Join(repo, "versions", id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reg := pyjson.NewObject()
	reg.Set("default", pyjson.String("A"))
	reg.Set("versions", vers)
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(pyjson.Dumps(reg, DumpDefault)))
}

// realCourse is the developer course bound to 4.15.6.20 in the
// niagara-research checkout ($HELPBUILD_PARITY_COURSE replaces it).
func realCourse(t *testing.T) string {
	if p := os.Getenv("HELPBUILD_PARITY_COURSE"); p != "" {
		return p
	}
	help := niagaraHelpData()
	if help == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(help), "sources", "n4-dev-course")
}

// TestParityCourseReal plans and copies the real course (only read) into
// twin repositories.
func TestParityCourseReal(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_COURSE")
	driver(t)
	src := realCourse(t)
	if explicit := os.Getenv("HELPBUILD_PARITY_COURSE"); explicit != "" {
		if countFiles(explicit, func(string) bool { return true }) == 0 {
			t.Fatalf("HELPBUILD_PARITY_COURSE=%q is no folder holding files: nothing to compare", explicit)
		}
	} else if src == "" || !pycompat.IsDir(src) {
		t.Skipf("developer course %q not found: real course parity not checked", src)
	}
	coursePlanParity(t, "n4-dev-course", src)
	dir := t.TempDir()
	pyRepo, goRepo := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	for _, repo := range []string{pyRepo, goRepo} {
		writeCourseRegistry(t, repo, map[string]string{"A": src})
	}
	courseStep(t, "n4-dev-course", pyRepo, goRepo, "versions/A")
	logTreeSize(t, filepath.Join(goRepo, "versions", "A"))
}

// onBoth applies one tree mutation to the Python and the Go data root.
func onBoth(t *testing.T, roots []string, f func(root string)) {
	t.Helper()
	for _, r := range roots {
		f(r)
	}
}
