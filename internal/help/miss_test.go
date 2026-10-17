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
