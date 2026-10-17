package help

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// TestCannotSearchPaths pins the exit-2 paths that print ERROR (or the
// not-ported note) on stderr and nothing on stdout.
func TestCannotSearchPaths(t *testing.T) {
	missing := func(dir, name string) string {
		return "ERROR: index indexes/" + name + " is missing under " + dir +
			"; rebuild the indexes with: python3 tools/niagara_help.py rebuild-all\n"
	}
	tests := []struct {
		name       string
		classIndex string // class-index.json of the checkout
		args       []string
		stderr     func(dir string) string
	}{
		{"guide needs guides-index.json", "{}", []string{"guide", "bacnet"},
			func(dir string) string { return missing(dir, "guides-index.json") }},
		{"source --grep waits for the Python-regex package", "{}", []string{"source", "BX", "--grep", "x"},
			func(string) string { return "nh: 'source --grep' is not ported yet\n" }},
		{"others item that is not an object", `{"BX": {"package": "p", "type": "class", "others": ["oops"]}}`,
			[]string{"class", "BX"},
			func(string) string { return "ERROR: malformed class index: \"others\"[0] of BX is not an object\n" }},
		{"others that is not a list", `{"BX": {"package": "p", "type": "class", "others": null}}`,
			[]string{"search", "B*"},
			func(string) string { return "ERROR: malformed class index: \"others\" of BX is not a list\n" }},
		{"malformed others under package", `{"BX": {"package": "p", "others": [1]}}`,
			[]string{"package", "--all"},
			func(string) string { return "ERROR: malformed class index: \"others\"[0] of BX is not an object\n" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := repo(t, twoVersions)
			write(t, filepath.Join(dir, "indexes", "class-index.json"), tt.classIndex)
			args := append([]string{"--niagara-version", "1.0"}, tt.args...)
			out, errs, code := run(t, dir, args...)
			if want := tt.stderr(dir); code != 2 || out != "" || errs != want {
				t.Errorf("code %d stdout %q stderr %q; want 2, \"\", %q", code, out, errs, want)
			}
		})
	}
}

func TestTypoSimilarityEmptyStrings(t *testing.T) {
	if got := typoSimilarity("", ""); got != 0 || math.IsNaN(got) {
		t.Errorf("typoSimilarity(\"\", \"\") = %v, want 0", got)
	}
	if got := suggest([]string{"", "a", ""}, "", 5); len(got) != 0 {
		t.Errorf("suggest for an empty query = %q, want none (an empty name is no typo of it)", got)
	}
}

// TestStrListCopiesTheIndex guards the cached indexes: callers sort the
// lists strList returns (interface-hierarchy children, compatible subtypes).
func TestStrListCopiesTheIndex(t *testing.T) {
	v, err := index.Decode([]byte(`{"subclasses": ["b", "a"]}`))
	if err != nil {
		t.Fatal(err)
	}
	o := v.(*index.Object)
	got := strList(o, "subclasses")
	got[0] = "changed"
	if again := strList(o, "subclasses"); again[0] != "b" {
		t.Errorf("strList shares the index slice: %q", again)
	}
}
