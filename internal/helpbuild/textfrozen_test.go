package helpbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/helpbuild/pdftext"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The S3 PDF extractor (pdf_extractor.main) on the index states that never
// convert a PDF, and the S4 course plan and extractor (course.plan,
// course_extractor.main) on a synthetic course, against their frozen Python
// output (frozen_test.go).

// elapsedLine is the extractor's run-time line, the one volatile line.
var elapsedLine = regexp.MustCompile(`(?m)^Time:              \d+\.\d s$`)

// toolLines turns a tool's streams and outcome into lines:
// each stream's lines, then "exit N" or "raise" (Go has no exception
// classes: the decision is compared).
func toolLines(root string, streams map[string]string, names []string, outcome string) []string {
	var out []string
	for _, n := range names {
		s := elapsedLine.ReplaceAllString(streams[n], "Time:              <t> s")
		for _, l := range strings.Split(strings.ReplaceAll(s, root, "<root>"), "\n") {
			out = append(out, fmt.Sprintf("%s %q", n, l))
		}
	}
	return append(out, outcome)
}

// syntheticDocs writes the docs/ tree of the synthetic install: text over
// two pages with an unmapped bullet, short and blank PDFs, a stem
// collision, hardware, top-level, nested, non-ASCII and punctuated names,
// a corrupt PDF, one whose text is not UTF-8 and one that times out (fake
// pdftotext side files), a symlinked folder and a file that is not a PDF.
func syntheticDocs(t *testing.T, docs string) {
	long := "Alarm classes route every alarm to its recipients and consoles."
	pdfs := map[string][]byte{
		"docAlarms/Alarms Guide.pdf":   testkit.MakePDF([]string{long, "72 \xb0F is the limit"}, []string{"Steps:", "\x80 Disconnect the LAN.", " \x80x stays"}),
		"docAlarms/A&B: notes.pdf":     testkit.MakePDF([]string{long}),
		"docX/Same Name.pdf":           testkit.MakePDF([]string{long, "first"}),
		"docY/Same_Name.PDF":           testkit.MakePDF([]string{long, "second"}),
		"docJ9Install/J9 Mounting.pdf": testkit.MakePDF([]string{"Mount the J9 on a DIN rail; wire the 24 V supply first."}),
		"Top Level.pdf":                testkit.MakePDF([]string{long, "top"}),
		"docNested/sub/deep.pdf":       testkit.MakePDF([]string{long, "deep"}),
		"docGuía/Guía ñ.pdf":           testkit.MakePDF([]string{"Gu\xeda del m\xf3dulo: se\xf1ales y alarmas del sistema completo."}),
		"docShort/short.pdf":           testkit.MakePDF([]string{"tiny"}),
		"docShort/blank.pdf":           testkit.MakePDF(nil),
		"docBad/corrupt.pdf":           []byte("%PDF-1.4\nnot really a PDF\n"),
		"docBad/latin1.pdf":            testkit.MakePDF([]string{"never read"}),
		"docBad/slow.pdf":              testkit.MakePDF([]string{"never read"}),
	}
	for rel, data := range pdfs {
		testkit.WriteFile(t, filepath.Join(docs, rel), data)
	}
	testkit.WriteFile(t, filepath.Join(docs, "docBad", "latin1.pdf.out"), []byte("72 \xb0F"))
	testkit.WriteFile(t, filepath.Join(docs, "docBad", "slow.pdf.sleep"), nil)
	testkit.WriteFile(t, filepath.Join(docs, "docAlarms", "readme.txt"), []byte("not a PDF"))
	if err := os.Symlink(filepath.Join(docs, "docAlarms"), filepath.Join(docs, "linked")); err != nil {
		t.Fatal(err)
	}
}

// TestPDFIndexStatesFrozen: pdf_extractor on the index states that never
// convert a PDF, so the output does not depend on the pdftotext build: an
// unregistered root without and with listed manuals, a corrupt index with
// nothing to rebuild from, an index that is not an object or not UTF-8, an
// install that is not mounted, a docs dir without PDFs, and pdftotext not
// on PATH. Per step: stdout and the outcome, then the tree delta of the
// repository. The conversions themselves are pdfindex_test.go's (fake
// pdftotext).
func TestPDFIndexStatesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	real := testkit.NeedPdftotext(t)
	dir := t.TempDir()
	// pdftotext on PATH, as the parity run had it; no step here converts a
	// PDF, so the build of the binary does not matter.
	t.Setenv("PATH", testkit.FakeBin(t, dir, real)+string(os.PathListSeparator)+os.Getenv("PATH"))
	install := filepath.Join(dir, "install")
	syntheticDocs(t, filepath.Join(install, "docs"))
	emptyDocs := filepath.Join(dir, "empty-docs")
	testkit.WriteFile(t, filepath.Join(emptyDocs, "notes.txt"), []byte("no PDF here"))
	repo := filepath.Join(dir, "go")
	writeRegistry(t, repo, map[string][3]string{
		"1.0": {"synthetic", install, "versions/1.0"},
		"9.9": {"not mounted", install + "-missing", "versions/9.9"},
	})
	for _, b := range []string{"fresh", "fresh2", "versions/1.0", "versions/9.9"} {
		if err := os.MkdirAll(filepath.Join(repo, b), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tree := testkit.TreeLines(t, repo)
	n := 0
	step := func(what, base, docs string) {
		t.Helper()
		res, err := pdftext.ExtractPDFs(filepath.Join(repo, base), pdftext.PDFOptions{DocsDir: docs})
		outcome := fmt.Sprintf("exit %d", res.Status)
		if err != nil {
			outcome = "raise"
		}
		lines := append(toolLines(repo, map[string]string{"stdout": pdfStdout(res)}, []string{"stdout"}, outcome), "tree:")
		cur := testkit.TreeLines(t, repo)
		n++
		testkit.CheckFrozen(t, fmt.Sprintf("pdf/%d-%s", n, testkit.FrozenName(what)), append(lines, testkit.TreeDelta(tree, cur)...))
		tree = cur
	}
	index := func(base, text string) {
		testkit.WriteFile(t, filepath.Join(repo, base, "indexes", "pdf-index.json"), []byte(text))
		tree = testkit.TreeLines(t, repo)
	}
	step("unregistered root", "fresh", "")
	index("fresh", `{"x": {"pages": 1}}`)
	step("unregistered root with manuals", "fresh", "")
	index("fresh", `{"x": 1,`)
	step("corrupt index, nothing to rebuild from", "fresh", "")
	index("fresh", "[1]")
	step("index not an object", "fresh", "")
	index("fresh", "\xff{}")
	step("index not UTF-8", "fresh", "")
	index("versions/9.9", "{\"x\": 1}\r\n")
	step("install not mounted", "versions/9.9", "")
	index("fresh2", `{"x": 1, "y": 2}`)
	step("docs dir holds no PDF", "fresh2", emptyDocs)

	t.Setenv("PATH", t.TempDir()) // no pdftotext
	step("pdftotext not on PATH", "versions/1.0", "")
}

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
	testkit.WriteTree(t, src, courseFixture)
	testkit.WriteTree(t, src, courseExtras)
	if err := os.Symlink(filepath.Join(src, "solutions"), filepath.Join(src, "course", "linked-solutions")); err != nil {
		t.Fatal(err)
	}
}

// goPlanLines is CoursePlan as lines: per file its path, kind, destination
// and "(course)" exclude rule.
func goPlanLines(t *testing.T, src string) []string {
	rules := mustDefault(t)
	var out []string
	for _, r := range CoursePlan(src, rules) {
		rule := ""
		if x := rules.ExcludeRule(CoursePseudoJar, r.Rel); x != nil {
			rule = x.ID
		}
		out = append(out, testkit.DumpLine(testkit.Str(r.Rel), testkit.OptStr(r.Kind), testkit.OptStr(r.Dest), testkit.OptStr(rule)))
	}
	return out
}

// TestCoursePlanFrozen: every file of the synthetic course with its plan
// row and "(course)" exclude rule.
func TestCoursePlanFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	src := filepath.Join(t.TempDir(), "course-src")
	syntheticCourse(t, src)
	testkit.CheckFrozen(t, "course/plan", goPlanLines(t, src))
}

// TestCourseExtractorFrozen: course_extractor on a course bound by a
// relative and by an absolute course_dir, unbound (earlier copies removed,
// a symlinked copy refused), bound to a missing folder, rebuilt after the
// course changed, on an unregistered root, with an unreadable file and a
// named pipe in the course, and with a registry that is not JSON. Per step:
// stdout, stderr and the outcome, then the tree delta of the repository.
func TestCourseExtractorFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "course-src")
	syntheticCourse(t, src)
	repo := filepath.Join(dir, "go")
	writeCourseRegistry(t, repo, map[string]string{
		"A": "../course-src", "B": "", "C": "../no-course", "D": src,
	})
	tree := testkit.TreeLines(t, repo)
	n := 0
	courseStep := func(what, rel string) {
		t.Helper()
		out, err := CourseExtractor(filepath.Join(repo, rel), mustDefault(t))
		outcome := fmt.Sprintf("exit %d", out.Status)
		if err != nil {
			outcome = "raise"
		}
		join := func(ls []string) string {
			if len(ls) == 0 {
				return ""
			}
			return strings.Join(ls, "\n") + "\n"
		}
		lines := append(toolLines(repo, map[string]string{"stdout": join(out.Stdout), "stderr": join(out.Stderr)},
			[]string{"stdout", "stderr"}, outcome), "tree:")
		cur := testkit.TreeLines(t, repo)
		n++
		testkit.CheckFrozen(t, fmt.Sprintf("course/%d-%s", n, testkit.FrozenName(what)), append(lines, testkit.TreeDelta(tree, cur)...))
		tree = cur
	}
	courseStep("bound by a relative course_dir", "versions/A")
	testkit.WriteFile(t, filepath.Join(repo, "versions", "B", "course-text", "stale.txt"), []byte("earlier binding"))
	testkit.WriteFile(t, filepath.Join(repo, "versions", "B", "devguide-examples", "course", "x", "Stale.java"), []byte("x"))
	testkit.WriteFile(t, filepath.Join(repo, "versions", "B", "devguide-examples", "other.txt"), []byte("not ours"))
	tree = testkit.TreeLines(t, repo)
	courseStep("unbound: earlier copies removed", "versions/B")
	b := filepath.Join(repo, "versions", "B")
	testkit.WriteFile(t, filepath.Join(b, "elsewhere", "x.txt"), []byte("behind a symlink"))
	if err := os.Symlink(filepath.Join(b, "elsewhere"), filepath.Join(b, "course-text")); err != nil {
		t.Fatal(err)
	}
	tree = testkit.TreeLines(t, repo)
	courseStep("unbound: rmtree refuses a symlink", "versions/B")
	courseStep("bound course_dir missing", "versions/C")
	courseStep("bound by an absolute course_dir", "versions/D")
	if err := os.Remove(filepath.Join(src, "text", "8 ORDs.txt")); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, filepath.Join(src, "text", "9 New.txt"), []byte("new page\n"))
	courseStep("rebuilt after the course changed", "versions/A")
	courseStep("unregistered root", "versions")
	if os.Geteuid() != 0 {
		locked := filepath.Join(src, "course", "Labs", "Lab1 First", "src", "BFirst.java")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		courseStep("a failed copy keeps the previous copies", "versions/A")
		if err := os.Chmod(locked, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if mkfifo, err := exec.LookPath("mkfifo"); err == nil {
		fifo := filepath.Join(src, "text", "pipe.txt")
		if out, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
			t.Fatalf("mkfifo: %v %s", err, out)
		}
		courseStep("a named pipe is refused, never opened", "versions/A")
		if err := os.Remove(fifo); err != nil {
			t.Fatal(err)
		}
	}
	testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), []byte("{"))
	tree = testkit.TreeLines(t, repo)
	courseStep("registry not JSON", "versions/A")
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
	testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(pyjson.Dumps(reg, pyfs.DumpDefault)))
}
