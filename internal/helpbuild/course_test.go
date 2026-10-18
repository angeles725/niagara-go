package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Unit tests of course.go, ported from niagara-help
// tests/test_course_source.py TestCourseBuild (the search and gate tests
// need the full-text index: S10).

// courseFixture is test_course_source.FILES: page text, course PDFs, lab
// and slide code, a dev/ readme, key material, a palette, a signing
// configuration, user solutions and a manifest.
var courseFixture = map[string]string{
	"text/Review Assignment 1.txt":                                   "Review Assignment 1\nBuild a thermostat\n\fPage two: wobbletherm setpoint logic\n",
	"text/8 ORDs.txt":                                                "ORDs\n\fNMessage link layer of the driver\n",
	"course/Review Assignments/Review Assignment 1.pdf":              "%PDF-1.4",
	"course/Slides/8 ORDs.pdf":                                       "%PDF-1.4",
	"course/Labs/Lab1 First/src/BFirst.java":                         "package lab;\npublic class BFirst { int quuxcounter; }\n",
	"course/Slide Code/slides/slides-rt/src/BSlide.java":             "public class BSlide {}\n",
	"course/Labs/Lab5 Code Signing/rootca.pem":                       "-----BEGIN CERTIFICATE-----",
	"course/Labs/Lab1 First/module.palette":                          "<palette/>",
	"niagara-4.15.6.20/dev/demo/readme.txt":                          "Demo readme fizzlecake\n",
	"niagara-4.15.6.20/dev/demo/security/niagara.signing.properties": "jarsigner.storepass=${jarsigner.storepass}\n",
	"niagara-4.15.6.20/dev/demo/src/BDemo.java":                      "public class BDemo {}\n",
	"solutions/cert-exam/BAnswer.java":                               "public class BAnswer {}\n",
	"MANIFEST.sha256":                                                "abc  x\n",
}

// courseRepo is test_course_source.make_repo: version A bound to the
// course by a relative course_dir, B without one; extra entries are added
// to the registry as given.
func courseRepo(t *testing.T, dir string, extra map[string]string) (repo string) {
	t.Helper()
	repo = filepath.Join(dir, "repo")
	writeTree(t, filepath.Join(dir, "course-src"), courseFixture)
	vers := pyjson.NewObject()
	add := func(id, courseDir string) {
		e := pyjson.NewObject()
		e.Set("label", pyjson.String(strings.ToLower(id)))
		e.Set("install_dir", pyjson.String("/nonexistent"))
		e.Set("data_root", pyjson.String("versions/"+id))
		if courseDir != "" {
			e.Set("course_dir", pyjson.String(courseDir))
		}
		vers.Set(id, e)
		if err := os.MkdirAll(filepath.Join(repo, "versions", id, "indexes"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	add("A", "../course-src")
	add("B", "")
	for _, id := range pyjson.SortedKeys(keysOfStr(extra)) {
		add(id, extra[id])
	}
	reg := pyjson.NewObject()
	reg.Set("default", pyjson.String("A"))
	reg.Set("versions", vers)
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(pyjson.Dumps(reg, DumpDefault)))
	return repo
}

func keysOfStr(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// copiedFiles lists every file under root, relative, "/"-separated.
func copiedFiles(t *testing.T, root string) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func TestCourseBuildCopiesTextAndCode(t *testing.T) { // test_text_and_code_are_copied_secrets_and_solutions_are_not
	dir := t.TempDir()
	a := filepath.Join(courseRepo(t, dir, nil), "versions", "A")
	counts, err := BuildCourse(a, mustDefault(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"course-text/8 ORDs.txt",
		"course-text/Review Assignment 1.txt",
		"course-text/niagara-4.15.6.20/dev/demo/readme.txt",
		"devguide-examples/course/course/Labs/Lab1 First/src/BFirst.java",
		"devguide-examples/course/course/Slide Code/slides/slides-rt/src/BSlide.java",
		"devguide-examples/course/niagara-4.15.6.20/dev/demo/src/BDemo.java",
	}
	if got := copiedFiles(t, a); !reflect.DeepEqual(got, want) {
		t.Errorf("copied %q\nwant %q", got, want)
	}
	if counts.Text != 3 || counts.Code != 3 || counts.CourseDir != filepath.Join(dir, "course-src") {
		t.Errorf("counts %+v", counts)
	}
}

func TestCourseUnboundBuildsNothing(t *testing.T) { // test_unbound_version_builds_nothing
	b := filepath.Join(courseRepo(t, t.TempDir(), nil), "versions", "B")
	writeFile(t, filepath.Join(b, "course-text", "old.txt"), []byte("left from an earlier binding"))
	counts, err := BuildCourse(b, mustDefault(t))
	if err != nil || counts != nil {
		t.Fatalf("BuildCourse(B) = %+v, %v", counts, err)
	}
	if isDir(filepath.Join(b, "course-text")) {
		t.Error("course-text/ of an unbound version kept")
	}
}

// A bound course_dir that does not exist touches nothing; a copy that
// fails keeps the previous copies (course_extractor's two skips).
func TestCourseMissingAndFailedCopy(t *testing.T) {
	dir := t.TempDir()
	repo := courseRepo(t, dir, map[string]string{"C": "../no-course"})
	c := filepath.Join(repo, "versions", "C")
	writeFile(t, filepath.Join(c, "course-text", "kept.txt"), []byte("kept"))
	_, err := BuildCourse(c, mustDefault(t))
	var missing *CourseDirMissingError
	if !errors.As(err, &missing) || missing.Error() != "course_dir not found: "+filepath.Join(dir, "no-course") {
		t.Fatalf("missing course: %v", err)
	}
	if !isRegular(filepath.Join(c, "course-text", "kept.txt")) {
		t.Error("a missing course_dir touched course-text/")
	}
	if runtime.GOOS == "windows" {
		t.Skip("mode 0 only sets the read-only attribute on Windows: the failed copy is not checked")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a 000 file: the failed copy is not checked")
	}
	a := filepath.Join(repo, "versions", "A")
	writeFile(t, filepath.Join(a, "course-text", "previous.txt"), []byte("previous copy"))
	locked := filepath.Join(dir, "course-src", "text", "8 ORDs.txt")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)
	out, err := CourseExtractor(a, mustDefault(t))
	if err != nil || out.Status != 2 || len(out.Stderr) != 1 ||
		!strings.Contains(out.Stderr[0], "[Errno 13] Permission denied: '"+locked+"'") {
		t.Fatalf("failed copy: %+v, %v", out, err)
	}
	if got := copiedFiles(t, a); !reflect.DeepEqual(got, []string{"course-text/previous.txt"}) {
		t.Errorf("after a failed copy: %q", got)
	}
}

func TestCourseTextDest(t *testing.T) {
	for in, want := range map[string]string{"text/a.txt": "a.txt", "text/sub/b.TXT": "sub/b.TXT",
		"niagara-1/dev/x/readme.md": "niagara-1/dev/x/readme.md.txt", "texts/a.txt": "texts/a.txt"} {
		if got := courseTextDest(in); got != want {
			t.Errorf("courseTextDest(%q) = %q, want %q", in, got, want)
		}
	}
}

// course._files order: a folder's files (sorted) before its subfolders
// (sorted); a symlinked folder is not entered.
func TestCourseFilesOrder(t *testing.T) {
	src := t.TempDir()
	upper := "B" // sorts before "c": a code point sort, not a case-insensitive one
	if runtime.GOOS == "windows" {
		upper = "D" // a case-insensitive file system cannot hold both B and b/
	}
	writeTree(t, src, map[string]string{"b/x": "", "a/z/y": "", "a/b": "", "c": "", upper: "", "outside/o": ""})
	if err := os.Symlink(filepath.Join(src, "outside"), filepath.Join(src, "linked")); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		t.Logf("no symlink privilege, the symlinked folder is not checked: %v", err)
	}
	var rels []string
	for _, f := range courseFiles(src) {
		rels = append(rels, f.rel)
	}
	if want := []string{upper, "c", "a/b", "a/z/y", "b/x", "outside/o"}; !reflect.DeepEqual(rels, want) {
		t.Errorf("courseFiles = %q, want %q", rels, want)
	}
}

// course_extractor.main()'s stdout lines and exit status for a bound, an
// unbound and a missing course, checked without the Python driver.
func TestCourseExtractorLines(t *testing.T) {
	dir := t.TempDir()
	repo := courseRepo(t, dir, map[string]string{"C": "../no-course"})
	for _, c := range []struct{ id, line string }{
		{"A", "course: 3 page text files -> course-text/, 3 example code files -> devguide-examples/course/ (from " +
			filepath.Join(dir, "course-src") + ")"},
		{"B", "course: no course bound to this version; skipped"},
		{"C", "course: course_dir not found: " + filepath.Join(dir, "no-course") + "; skipped (the course is " +
			"optional: course-text/ and devguide-examples/course/ keep their last copy)"},
	} {
		out, err := CourseExtractor(filepath.Join(repo, "versions", c.id), mustDefault(t))
		if err != nil || out.Status != 0 || len(out.Stderr) != 0 || !reflect.DeepEqual(out.Stdout, []string{c.line}) {
			t.Errorf("%s: CourseExtractor = %+v, %v\nwant stdout %q", c.id, out, err, c.line)
		}
	}
}

// A symlink in place of course-text/ is refused like shutil.rmtree
// (OSError(None, None, path)): nothing behind it is removed and
// course_extractor reports the copy failure (exit 2).
func TestRemoveCourseCopiesRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	b := filepath.Join(courseRepo(t, dir, nil), "versions", "B")
	target := filepath.Join(dir, "elsewhere")
	writeFile(t, filepath.Join(target, "keep.txt"), []byte("not ours"))
	link := filepath.Join(b, CourseTextDir)
	symlinkOrSkip(t, target, link)
	removed, err := RemoveCourseCopies(b)
	want := "[Errno None] None: " + pycompat.FSRepr(link)
	if err == nil || err.Error() != want || len(removed) != 0 {
		t.Errorf("RemoveCourseCopies = %q, %v; want the error %q", removed, err, want)
	}
	out, err := CourseExtractor(b, mustDefault(t))
	line := "ERROR: course: cannot copy the course into " + b + ": " + want +
		" (course-text/ and devguide-examples/course/ keep their last copy)"
	if err != nil || out.Status != 2 || len(out.Stdout) != 0 || !reflect.DeepEqual(out.Stderr, []string{line}) {
		t.Errorf("CourseExtractor = %+v, %v\nwant stderr %q", out, err, line)
	}
	if !isRegular(filepath.Join(target, "keep.txt")) {
		t.Error("the symlink's target was emptied")
	}
}
