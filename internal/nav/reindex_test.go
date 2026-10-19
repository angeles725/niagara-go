package nav

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

var ansiColour = regexp.MustCompile("\033\\[[0-9]+m")

// TestMainRoutesReindex: Main sends `reindex` to the index rebuild after the
// global --base-dir/-d in every argparse spelling, which then is its checkout:
// the default corpus is organized/ next to it. The command word comes from
// commandWord, so a value or an option is never taken for it.
func TestMainRoutesReindex(t *testing.T) {
	root := t.TempDir()
	nav := filepath.Join(root, "module-navigator")
	for _, d := range []string{filepath.Join(nav, "indexes"), filepath.Join(root, "organized")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NAV_CORPUS_BASE", "")
	want := "Corpus: " + filepath.Join(root, "organized") + "\n"
	for _, args := range [][]string{
		{"--base-dir", nav, "reindex", "--check"},
		{"-d", nav, "reindex", "--check"},
		{"--base-dir=" + nav, "reindex", "--check"},
		{"-d" + nav, "reindex", "--check"},
		{"--base", nav, "reindex", "--check"},
		{"-d", nav, "--", "reindex", "--check"},
		{"-d", "/elsewhere", "-d", nav, "reindex", "--check"},
		{"reindex", "--base-dir", nav, "--check"},
	} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		got := ansiColour.ReplaceAllString(out.String(), "")
		if code != cli.ExitFound || !strings.HasPrefix(got, want) || !strings.Contains(got, "Preflight OK.") || errb.Len() != 0 {
			t.Errorf("Main(%q): exit %d, stdout %q, stderr %q; want exit 0 and %q", args, code, got, errb.String(), want)
		}
	}
}

// TestMainReindexKeepsOtherLines: an empty --base-dir is reindex's own usage
// error, a -d that takes "reindex" as its value is module-nav's help (no
// command), a -d without a value is module-nav's usage error, and RunAt
// (nsearch's entry) never routes reindex.
func TestMainReindexKeepsOtherLines(t *testing.T) {
	for _, tc := range []struct {
		args           []string
		code           int
		stdout, stderr string // prefixes
	}{
		{[]string{"--base-dir=", "reindex"}, cli.ExitCannotSearch, "", "--base-dir needs a directory\n"},
		{[]string{"-d", "reindex"}, cli.ExitFound, "usage: module-nav [-h] [--base-dir DIR] <command> [<args>]\n", ""},
		{[]string{"-d", "--", "reindex"}, cli.ExitCannotSearch, "",
			"usage: module-nav [-h] [--base-dir DIR] <command> [<args>]\nmodule-nav: error: argument --base-dir/-d: expected one argument\n"},
	} {
		var out, errb bytes.Buffer
		code := Main(tc.args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		if code != tc.code || !strings.HasPrefix(out.String(), tc.stdout) || !strings.HasPrefix(errb.String(), tc.stderr) ||
			(tc.stdout == "") != (out.Len() == 0) || (tc.stderr == "") != (errb.Len() == 0) {
			t.Errorf("Main(%q): exit %d, stdout %q, stderr %q; want exit %d, stdout %q..., stderr %q...",
				tc.args, code, out.String(), errb.String(), tc.code, tc.stdout, tc.stderr)
		}
	}
	var out, errb bytes.Buffer
	if code := RunAt(t.TempDir(), []string{"reindex", "--check"}, cli.Stdio{Out: &out, Err: &errb}); code != cli.ExitCannotSearch ||
		!strings.Contains(errb.String(), "invalid choice: 'reindex'") {
		t.Errorf("RunAt reindex: exit %d, stderr %q", code, errb.String())
	}
}

// TestMainReindexNotRouted: "reindex" that is not the command word (an
// argument of class or search) and a line whose command is not reindex never
// reach the index rebuild: Main prints no "Corpus:" line and answers exactly
// as module-nav (run) does for the same line.
func TestMainReindexNotRouted(t *testing.T) {
	root := t.TempDir()
	nav := filepath.Join(root, "module-navigator")
	for _, d := range []string{filepath.Join(nav, "indexes"), filepath.Join(root, "organized")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NAV_CORPUS_BASE", "")
	rebuild := "Corpus: " + filepath.Join(root, "organized") + "\n"
	for _, args := range [][]string{
		{"-d", nav, "class", "reindex"},
		{"-d", nav, "class", "reindex", "--check"},
		{"--base-dir", nav, "class"},
		{"--base-dir", nav, "class", "Foo"},
		{"-d", nav, "search", "reindex"},
		{"-d", nav, "--", "search", "reindex", "--check"},
	} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		var wantOut, wantErr bytes.Buffer
		wantCode := run("", args, cli.Stdio{In: strings.NewReader(""), Out: &wantOut, Err: &wantErr})
		got := ansiColour.ReplaceAllString(out.String(), "")
		if strings.HasPrefix(got, rebuild) || code != wantCode || out.String() != wantOut.String() || errb.String() != wantErr.String() {
			t.Errorf("Main(%q): exit %d, stdout %q, stderr %q; want module-nav's exit %d, stdout %q, stderr %q",
				args, code, got, errb.String(), wantCode, wantOut.String(), wantErr.String())
		}
	}
}
