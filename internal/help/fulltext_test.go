package help

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// TestFulltextCommandsMatchPython runs find, guide-search, devguide-search
// and pdf over the full-text fixture checkout and compares them with what
// tools/niagara_help.py printed there (testdata/fulltext-cli, written by
// testdata/fulltext_gen.py; <ROOT> is the checkout).
func TestFulltextCommandsMatchPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/fulltext-cli/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string   `json:"name"`
		Args   []string `json:"args"`
		Exit   int      `json:"exit"`
		Stderr string   `json:"stderr"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(testdata, "fulltext-cli", c.Name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			out, errs, code := run(t, checkout, c.Args...)
			wantOut := strings.ReplaceAll(string(want), ftsfixture.Root, checkout)
			wantErr := strings.ReplaceAll(c.Stderr, ftsfixture.Root, checkout)
			if code != c.Exit || errs != wantErr || out != wantOut {
				t.Errorf("exit %d (want %d) stderr %q (want %q) stdout:\n%s\nwant:\n%s",
					code, c.Exit, errs, wantErr, out, wantOut)
			}
		})
	}
}

// TestFulltextWithoutIndexCannotSearch: with no up-to-date fulltext.sqlite
// the search commands exit 2 naming the Python rebuild command (intended
// difference: Python builds the index), while a query without words and
// pdf --no-fulltext never need the index.
func TestFulltextWithoutIndexCannotSearch(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, true)
	want := "ERROR: full-text index indexes/fulltext.sqlite under " + checkout + " is missing; this port " +
		"reads it but never builds it, rebuild it with: python3 tools/niagara_help.py " +
		"--niagara-version 1.0 find index\n"
	for _, args := range [][]string{
		{"find", "alarm"}, {"guide-search", "alarm"}, {"devguide-search", "servlet"}, {"pdf", "modbus"},
	} {
		out, errs, code := run(t, checkout, append([]string{"--niagara-version", "1.0"}, args...)...)
		if code != 2 || out != "" || errs != want {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, errs)
		}
	}
	if _, errs, code := run(t, checkout, "--niagara-version", "1.0", "find", "!!!"); code != 2 ||
		errs != "ERROR: query '!!!' has no searchable word\n" {
		t.Errorf("no-word query: exit %d stderr %q", code, errs)
	}
	if out, _, code := run(t, checkout, "--niagara-version", "1.0", "pdf", "modbus", "--no-fulltext"); code != 0 ||
		!strings.HasPrefix(out, "2 PDF manuals matching 'modbus':") {
		t.Errorf("--no-fulltext: exit %d stdout %q", code, out)
	}
}

func TestFtHeadIsPythonSliceEnd(t *testing.T) {
	for _, c := range [][3]int{{5, 3, 3}, {5, 9, 5}, {5, -1, 4}, {5, -9, 0}, {0, -1, 0}, {5, 0, 0}} {
		if got := ftHead(c[0], c[1]); got != c[2] {
			t.Errorf("ftHead(%d, %d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestFormatF0RoundsHalfToEven(t *testing.T) {
	for v, want := range map[any]string{json.Number("0.5"): "0", json.Number("1.5"): "2", json.Number("2.5"): "2",
		json.Number("120.5"): "120", json.Number("3"): "3", -0.4: "-0", 0: "0", "x": "x"} {
		if got := formatF0(v); got != want {
			t.Errorf("formatF0(%v) = %q, want %q", v, got, want)
		}
	}
}
