package help

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestS5CommandsMatchPython runs every case of testdata/s5-cli/cases.json on
// the testdata/s5 checkout and compares it with what tools/niagara_help.py
// printed there (testdata/s5-cli, written by
// testdata/s5_gen.py; <ROOT> is the checkout). A null stderr is a case where
// Python built its full-text index first: Go never builds it, so method
// orders without doc prominence and says so on stderr (intended difference;
// with no indexed text both orders are the same).
func TestS5CommandsMatchPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/s5-cli/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string   `json:"name"`
		Args   []string `json:"args"`
		Exit   int      `json:"exit"`
		Stderr *string  `json:"stderr"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("testdata", "s5"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "s5-cli", c.Name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			var out, errs string
			var code int
			if strings.HasPrefix(c.Name, "fuzzy-") {
				out, errs, code = runFuzzy(t, root, c.Args)
			} else {
				out, errs, code = run(t, root, c.Args...)
			}
			wantOut := strings.ReplaceAll(string(want), "<ROOT>", root)
			// The recall header prints the parse time, rounded to seconds.
			out = strings.Replace(out, "parsed in 1s)", "parsed in 0s)", 1)
			if code != c.Exit || out != wantOut {
				t.Errorf("exit %d (want %d) stdout:\n%s\nwant:\n%s", code, c.Exit, out, wantOut)
			}
			switch {
			case c.Stderr == nil:
				if !strings.HasSuffix(errs, "; results are not ordered by doc prominence\n") {
					t.Errorf("stderr %q, want the full-text unavailable note", errs)
				}
			case errs != strings.ReplaceAll(*c.Stderr, "<ROOT>", root):
				t.Errorf("stderr %q, want %q", errs, *c.Stderr)
			}
		})
	}
}

// runFuzzy runs the fuzzy search the recall oracle samples (the `fuzzy`
// command is retired in Go) for the arguments of a recorded `fuzzy QUERY
// [-n N]` case, mapping its outcome to the exit status the CLI would give.
func runFuzzy(t *testing.T, root string, args []string) (stdout, stderr string, code int) {
	t.Helper()
	i := 0
	for args[i] != "fuzzy" {
		i++
	}
	query, top := args[i+1], 15
	if len(args) > i+3 && args[i+2] == "-n" {
		if _, err := fmt.Sscan(args[i+3], &top); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	e := &Env{Out: &out, Err: &errb, RepoRoot: root, BaseDir: root, VersionID: "4.14.0.162"}
	switch err := e.fuzzy(query, top); {
	case err != nil:
		fmt.Fprintf(&errb, "ERROR: %v\n", err)
		code = 2
	case e.misses > 0:
		code = 1
	}
	return out.String(), errb.String(), code
}
