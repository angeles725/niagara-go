package corpusnav

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// corpusCommands is every command that reads the corpus, with arguments.
var corpusCommands = [][]string{
	{"find", "bacnet"}, {"find", "bacnet", "--in", "docs"}, {"find", "bacnet", "--in", "all"},
	{"grep", "bacnet"}, {"grep", "bacnet", "--in", "retros"}, {"show", "4"}, {"list"}, {"list", "--focus", "x"},
	{"by-marker", "CERT"}, {"by-focus", "x"}, {"connections", "4"}, {"graph"}, {"graph", "--json"},
	{"path", "4", "5"}, {"lint-refs"}, {"stats"}, {"selftest"}, {"semantic", "alarm"},
	{"semantic", "alarm", "--tfidf"}, {"rank", "alarm"},
}

func runMain(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// TestEmptyRootCannotSearch: a root with no block files is not the corpus. Every lookup
// there exits 2 and names the root on stderr instead of reporting "no matches" over
// 0 files (TOOLING-GUIDELINES principle 2), and nothing is written under it.
func TestEmptyRootCannotSearch(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CORPUS_NAV_ROOT", root)
	want := "corpus-nav: cannot search: no block files under " + root +
		" (set CORPUS_NAV_ROOT to the niagara-research checkout)\n"
	for _, args := range corpusCommands {
		code, out, errs := runMain(args...)
		if code != 2 || out != "" || errs != want {
			t.Errorf("%v: exit %d\nstdout %q\nstderr %q", args, code, out, errs)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("the empty root gained %d entries (first %s)", len(entries), entries[0].Name())
	}
}

// TestDefaultRootWithoutBlocks: with CORPUS_NAV_ROOT unset the root is the executable's
// grandparent (an install directory, not a checkout); the message names it.
func TestDefaultRootWithoutBlocks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CORPUS_NAV_ROOT", "")
	saved := defaultRoot
	defaultRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { defaultRoot = saved })
	code, out, errs := runMain("find", "bacnet")
	want := "corpus-nav: cannot search: no block files under " + root +
		" (set CORPUS_NAV_ROOT to the niagara-research checkout)\n"
	if code != 2 || out != "" || errs != want {
		t.Errorf("exit %d\nstdout %q\nstderr %q", code, out, errs)
	}
}

// TestLintRefsFilesNeedNoCorpus: lint-refs FILE... reads only the files it is given,
// so it still runs from an empty root.
func TestLintRefsFilesNeedNoCorpus(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CORPUS_NAV_ROOT", root)
	file := filepath.Join(root, "note.md")
	if err := os.WriteFile(file, []byte("cites [B4]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := runMain("lint-refs", file); code != 0 || errs != "" ||
		!strings.HasPrefix(out, "\n0 non-canonical citation(s) in 0 file(s)") {
		t.Errorf("exit %d\nstdout %q\nstderr %q", code, out, errs)
	}
}

// TestEmptyScopeCannotSearch: in a corpus whose docs or retros directory holds no file,
// find and grep over that scope exit 2 naming the scope instead of "no matches".
func TestEmptyScopeCannotSearch(t *testing.T) {
	for _, scope := range []string{"docs", "retros"} {
		for _, cmd := range []string{"find", "grep"} {
			c, out, errb, root := copyCorpus(t)
			if err := os.RemoveAll(filepath.Join(root, scope)); err != nil {
				t.Fatal(err)
			}
			var code int
			if cmd == "find" {
				code = c.cmdFind("alarm", scope, 50, false)
			} else {
				code = c.cmdGrep("alarm", scope, 50)
			}
			want := "corpus-nav: cannot search: no files in scope " + scope + " under " + root + "\n"
			if code != 2 || out.String() != "" || errb.String() != want {
				t.Errorf("%s --in %s: exit %d\nstdout %q\nstderr %q", cmd, scope, code, out.String(), errb.String())
			}
		}
	}
}
