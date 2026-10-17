package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMisses checks the honest-zero block of each lookup against the Python
// output frozen in testdata/misses, where <TESTDATA> stands for the absolute
// data root (which is why misses are not golden cases).
func TestMisses(t *testing.T) {
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		file string
		args []string
	}{
		{"class-suggest", []string{"class", "BWebServlte"}},
		{"class-qualified-miss", []string{"class", "nope.BZzz"}},
		{"search", []string{"search", "Zz*"}},
		{"search-jdk", []string{"search", "Zz*", "--jdk"}},
		{"package", []string{"package", "zzz"}},
		{"hierarchy", []string{"hierarchy", "BZzz"}},
		{"implementors", []string{"implementors", "BIZzz"}},
		{"xref", []string{"xref", "BZzz"}},
		{"interface-hierarchy", []string{"interface-hierarchy", "Zzz"}},
		{"slots", []string{"slots", "Zzz"}},
		{"slots-by-type", []string{"slots", "--by-type", "BZzz"}},
		{"inherited-slots", []string{"inherited-slots", "Zzz"}},
		{"inherited-methods", []string{"inherited-methods", "Zzz"}},
		{"overrides", []string{"overrides", "Zzz"}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, "misses", c.file+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.ReplaceAll(string(raw), "<TESTDATA>", root)
			args := append([]string{"--niagara-version", "4.14.0.162"}, c.args...)
			out, errs, code := run(t, root, args...)
			if code != 1 || out != want || errs != "" {
				t.Errorf("code %d stderr %q stdout:\n%s\nwant:\n%s", code, errs, out, want)
			}
		})
	}
}

func TestMissingRequiredIndex(t *testing.T) {
	var out, errb strings.Builder
	e := &Env{Out: &out, Err: &errb, BaseDir: t.TempDir()}
	_, err := e.loadIndex("inheritance.json", true)
	want := "index indexes/inheritance.json is missing under " + e.BaseDir +
		"; rebuild the indexes with: python3 tools/niagara_help.py rebuild-all"
	if err == nil || err.Error() != want {
		t.Errorf("err %v, want %q", err, want)
	}
	if o, err := e.loadIndex("inheritance.json", false); err != nil || o.Len() != 0 {
		t.Errorf("optional missing index: %v %v", o, err)
	}
}

func TestClassJDKIsNotPorted(t *testing.T) {
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, root, "--niagara-version", "4.14.0.162", "class", "String", "--jdk")
	if code != 2 || out != "" || errs != "nh: 'class --jdk' is not ported yet\n" {
		t.Errorf("code %d out %q err %q", code, out, errs)
	}
}

func TestCleanClassName(t *testing.T) {
	for in, want := range map[string]string{"Cursor&lt;E&gt; ": "Cursor", "Map&lt;K, V&gt;.Entry": "Map.Entry", "BPlain": "BPlain"} {
		if got := cleanClassName(in); got != want {
			t.Errorf("cleanClassName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEmptyIndexMisses covers the archetypes and orphans misses, which need
// an empty (but present) inheritance or xref index.
func TestEmptyIndexMisses(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"inheritance.json", "class-index.json", "xref-index.json"} {
		write(t, filepath.Join(dir, "indexes", name), "{}")
	}
	for _, c := range []struct {
		run  func(e *Env) error
		want string
	}{
		{func(e *Env) error { return e.Archetypes(ArchetypesParams{}) },
			"No match for '' (archetypes) in this scope.\n  searched: Niagara unregistered (" + dir +
				") -- inheritance.json: 0 entries\n"},
		{func(e *Env) error { return e.Orphans(OrphansParams{Top: 30}) },
			"No match for '' (orphans) in this scope.\n  searched: Niagara unregistered (" + dir +
				") -- xref-index.json: 0 entries\n"},
	} {
		var out strings.Builder
		e := &Env{Out: &out, BaseDir: dir}
		if err := c.run(e); err != nil || e.misses != 1 || !strings.HasPrefix(out.String(), c.want) {
			t.Errorf("err %v misses %d out:\n%s", err, e.misses, out.String())
		}
	}
}
