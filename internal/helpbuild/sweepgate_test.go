package helpbuild

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// GateCourse on a bound course (test_course_source's fixture): every
// copied text and code file is looked up in the full-text index under its
// destination with the course-text/ or devguide-examples/ prefix cut off
// and put back as "/"-separated, PDFs pair with text/<stem>.txt, the rest
// falls to the (course) rules. It is also the one run of the Dest slicing
// (a destination shorter than its prefix would panic there).
func TestGateCourseBoundCourse(t *testing.T) {
	dir := t.TempDir()
	repo := courseRepo(t, dir, map[string]string{"C": "../no-course"})
	indexed := map[string]bool{
		"course-text/8 ORDs.txt":                                                      true,
		"course-text/Review Assignment 1.txt":                                         true,
		"course-text/niagara-4.15.6.20/dev/demo/readme.txt":                           true,
		"devguide-examples/course/course/Labs/Lab1 First/src/BFirst.java":             true,
		"devguide-examples/course/course/Slide Code/slides/slides-rt/src/BSlide.java": true,
	}
	docs := func(string) (map[string]bool, error) { return indexed, nil }
	rules := mustDefault(t)

	rep, err := GateCourse(filepath.Join(repo, "versions", "A"), rules, docs)
	if err != nil || rep == nil {
		t.Fatalf("GateCourse(A) = %+v, %v", rep, err)
	}
	if rep.CourseDir != filepath.Join(dir, "course-src") || rep.Text != 3 || rep.TextIndexed != 3 ||
		rep.Code != 3 || rep.CodeIndexed != 2 || rep.PDFs != 2 || rep.Paired != 2 {
		t.Errorf("counts %+v", rep)
	}
	excluded := 0
	for _, n := range rep.Excluded {
		excluded += n
	}
	if excluded != 7 { // two paired PDFs, key material, a palette, a signing configuration, solutions, the manifest
		t.Errorf("excluded %d entries, want 7: %v", excluded, rep.Excluded)
	}
	want := [][2]string{{"niagara-4.15.6.20/dev/demo/src/BDemo.java",
		"copied to " + filepath.Join("devguide-examples", "course", "niagara-4.15.6.20", "dev", "demo", "src", "BDemo.java") + " but not indexed"}}
	if !reflect.DeepEqual(rep.Unclassified, want) {
		t.Errorf("unclassified %q\nwant %q", rep.Unclassified, want)
	}

	if rep, err := GateCourse(filepath.Join(repo, "versions", "B"), rules, docs); rep != nil || err != nil {
		t.Errorf("GateCourse(B), no course bound = %+v, %v; want nil, nil", rep, err)
	}
	rep, err = GateCourse(filepath.Join(repo, "versions", "C"), rules, docs)
	missing := filepath.Join(dir, "no-course")
	if err != nil || rep == nil || !reflect.DeepEqual(rep.Unclassified, [][2]string{{missing, "course_dir not found"}}) {
		t.Errorf("GateCourse(C), missing course_dir = %+v, %v", rep, err)
	}
}

// A registry the gate cannot read is an error with exit status 1 and
// nothing printed (the CLI reports the error), not an unavailable install.
func TestRunSweepRegistryErrorExitsOne(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "versions", "1.0")
	writeFile(t, filepath.Join(repo, index.RegistryRel), []byte(`{"default": "1.0", "versions": []}`))
	var buf bytes.Buffer
	status, err := RunSweep(repo, root, "1.0", &buf, SweepOptions{})
	if status != 1 || err == nil || buf.Len() != 0 {
		t.Errorf("RunSweep = %d, %v, printed %q; want 1, the registry error, nothing", status, err, buf.String())
	}
}
