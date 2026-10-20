package registry

import (
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// Identify, InstallDir and CourseDir skip what Python catches and fail on
// what makes it raise.
func TestRegistryLookups(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	reg := func(text string) {
		testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(text))
	}
	root := filepath.Join(repo, "versions", "A")
	reg(`{"versions": {"bad": 5, "nodr": {"install_dir": "/x"}, "num": {"data_root": 3},` +
		` "A": {"data_root": "versions/./A/", "install_dir": "/inst", "course_dir": "../c"}}}`)
	if vid, err := Identify(root); vid != "A" || err != nil {
		t.Errorf("identify = %q, %v", vid, err)
	}
	if d, err := InstallDir(root); d != "/inst" || err != nil {
		t.Errorf("installDir = %q, %v", d, err)
	}
	if d, err := CourseDir(root); d != filepath.Join(dir, "c") || err != nil {
		t.Errorf("courseDir = %q, %v", d, err)
	}
	for text, wantErr := range map[string]bool{
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": ""}}}`: false,
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": 0}}}`:  false,
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": 5}}}`:  true,
		`{"versions": {"A": {"data_root": "versions/A", "course_dir": [1]}}}`: true,
		`{"versions": ["A"]}`: true, // identify skips it; course_dir's .get raises
		`{"versions": 5}`:     true,
		`[1]`:                 true,
		`{}`:                  false,
		`{"versions": null}`:  true,
	} {
		reg(text)
		_, err1 := InstallDir(root)
		_, err2 := CourseDir(root)
		if got := err1 != nil || err2 != nil; got != wantErr {
			t.Errorf("%s: installDir %v, courseDir %v; want an error %v", text, err1, err2, wantErr)
		}
	}
}
