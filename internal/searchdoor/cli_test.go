package searchdoor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitLicense(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		ok   bool
		rest []string
	}{
		{"route word first", []string{"license", "bacnet"}, true, []string{"bacnet"}},
		{"after a value option", []string{"--limit", "3", "license", "x"}, true, []string{"--limit", "3", "x"}},
		{"after a flag", []string{"--and", "license", "x"}, true, []string{"--and", "x"}},
		{"alone is free text", []string{"license"}, false, []string{"license"}},
		{"after -- is query text", []string{"--", "license", "bacnet"}, false, []string{"--", "license", "bacnet"}},
		{"route then --", []string{"license", "--", "bacnet"}, true, []string{"--", "bacnet"}},
		{"not first positional", []string{"bacnet", "license"}, false, []string{"bacnet", "license"}},
		{"a lone dash is positional", []string{"-", "license"}, false, []string{"-", "license"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, rest := splitLicense(tt.argv)
			if ok != tt.ok || !reflect.DeepEqual(rest, tt.rest) {
				t.Errorf("splitLicense(%q) = %v %q, want %v %q", tt.argv, ok, rest, tt.ok, tt.rest)
			}
		})
	}
}

func TestDefaultPathsHonourEachVariable(t *testing.T) {
	t.Setenv("NSEARCH_ROOT", "/r")
	t.Setenv("NSEARCH_CORPUS_NAV", "")
	t.Setenv("NSEARCH_NIAGARA_HELP", "")
	t.Setenv("NSEARCH_MODULE_NAV", "x/nav.py")
	t.Setenv("NSEARCH_COURSE_ROOT", "")
	want := Paths{Blocks: "/r/tools/corpus-nav.py", Docs: "/r/niagara-help/tools/niagara_help.py",
		Code: "x/nav.py", Course: "/r/sources/n4-dev-course"}
	if got := defaultPaths(); got != want {
		t.Errorf("defaultPaths() = %+v, want %+v", got, want)
	}
}

func TestRootIsFoundAboveTheExecutableThenTheWorkingDirectory(t *testing.T) {
	t.Setenv("NSEARCH_ROOT", "")
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "tools", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "tools", "corpus-nav.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	saved := executable
	t.Cleanup(func() { executable = saved })

	executable = func() (string, error) { return filepath.Join(checkout, "tools", "sub", "nsearch"), nil }
	t.Chdir(elsewhere)
	if got := root(); got != checkout {
		t.Errorf("root from the executable = %q, want %q", got, checkout)
	}
	executable = func() (string, error) { return filepath.Join(elsewhere, "bin", "nsearch"), nil }
	t.Chdir(filepath.Join(checkout, "tools"))
	if got := root(); got != checkout {
		t.Errorf("root from the working directory = %q, want %q", got, checkout)
	}
	t.Chdir(elsewhere)
	if got := root(); got != elsewhere {
		t.Errorf("fallback root = %q, want the executable directory's parent %q", got, elsewhere)
	}
}
