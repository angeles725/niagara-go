package checkcoverage

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are niagara-research tools/tests/fixtures/golden-cli/check-coverage.json and
// check-coverage-empty.json (the Python output, converted by the retired testdata/convert_golden.py); the
// repository root is the case's working directory (testdata/corpus).

func useRoot(t *testing.T, root func() (string, error)) {
	t.Helper()
	saved := repoRoot
	repoRoot = root
	t.Cleanup(func() { repoRoot = saved })
}

func TestGolden(t *testing.T) {
	useRoot(t, os.Getwd)
	golden.Run(t, "testdata", Main)
}

// TestGoldenNoBlocks runs check-coverage-empty.json: the root is the case's empty {tmp},
// passed in GOLDEN_REPO_ROOT (in process only: a binary resolves its own root).
func TestGoldenNoBlocks(t *testing.T) {
	if os.Getenv(golden.BinEnv) != "" {
		t.Skip("the root is injected in process; " + golden.BinEnv + " resolves its own")
	}
	useRoot(t, func() (string, error) { return os.Getenv("GOLDEN_REPO_ROOT"), nil })
	golden.Run(t, "testdata/empty", Main)
}

// TestNoBlocksExits3 is check-coverage-empty.json: zero block files means the gate did
// not run, so "clear" would be a lie.
func TestNoBlocksExits3(t *testing.T) {
	root := t.TempDir()
	useRoot(t, func() (string, error) { return root, nil })
	for _, args := range [][]string{{"zzqq"}, {"--json", "zzqq"}} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		want := "check-coverage: no *bloque*.md block files under " + root + " -- gate not run\n"
		if code != 3 || out.Len() != 0 || errb.String() != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errb.String())
		}
	}
}

func TestJSONReportEscapes(t *testing.T) {
	name := "a\"b\\c\n\x01ñ \xffabc"
	results := map[string]*coverage{name: {body: []hit{{"B1", "ñ"}}}}
	want := "{\n  \"scanned_blocks\": 1,\n  \"names\": {\n" +
		"    \"a\\\"b\\\\c\\n\\u0001ñ �abc\": {\n" +
		"      \"subject\": [],\n      \"body\": [\n        {\n" +
		"          \"block\": \"B1\",\n          \"h1\": \"ñ\"\n        }\n      ]\n    }\n  },\n" +
		"  \"verdict\": \"clear\"\n}"
	if got := jsonReport(results, []string{name}, 1, 0, false); got != want {
		t.Errorf("jsonReport =\n%s\nwant\n%s", got, want)
	}
}

// unreadableCorpus is a root with one readable block and one block path that cannot be
// read (a directory named like a block: EISDIR, even for root).
func unreadableCorpus(t *testing.T, body string) (root, bad string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "niagara-mental-model-bloque4.md"), []byte("# Alarms\n"+body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad = filepath.Join(root, "niagara-mental-model-bloque9.md")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	useRoot(t, func() (string, error) { return root, nil })
	return root, bad
}

// dirReason is why Python's open() fails on a directory: EISDIR, or EACCES from the
// Windows C runtime.
func dirReason() string {
	if runtime.GOOS == "windows" {
		return "Permission denied"
	}
	return "Is a directory"
}

// TestUnreadableBlockIsNamedAndNeverClear: Python skips a block it cannot read and may
// then print "clear" (exit 0). Intended difference: each unreadable block is named on
// stderr, and a run that would otherwise be clear is incomplete (exit 3, the
// gate-not-run code) instead; coverage found elsewhere still exits 2.
func TestUnreadableBlockIsNamedAndNeverClear(t *testing.T) {
	_, bad := unreadableCorpus(t, "nothing here")
	warn := "check-coverage: cannot read " + bad + ": " + dirReason() + "\n"
	var out, errb bytes.Buffer
	code := Main([]string{"zzqq"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	wantOut := "prior-coverage gate — scanned 2 block(s), 1 unreadable\n\n" +
		"  [CLEAR]   zzqq — no prior block mentions it. Safe to open as net-new.\n" +
		"VERDICT: incomplete. 1 block file(s) could not be read (named on stderr), so \"clear\" is not proven.\n"
	if code != 3 || out.String() != wantOut || errb.String() != warn {
		t.Errorf("text: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	code = Main([]string{"--json", "zzqq"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	wantJSON := "{\n  \"scanned_blocks\": 2,\n  \"names\": {\n    \"zzqq\": {\n      \"subject\": [],\n" +
		"      \"body\": []\n    }\n  },\n  \"unreadable_blocks\": 1,\n  \"verdict\": \"incomplete\"\n}\n"
	if code != 3 || out.String() != wantJSON || errb.String() != warn {
		t.Errorf("json: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}

func TestUnreadableBlockKeepsCoverageVerdict(t *testing.T) {
	_, bad := unreadableCorpus(t, "honeywellSpyderTool")
	var out, errb bytes.Buffer
	code := Main([]string{"honeywellSpyderTool"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if code != 2 || !strings.Contains(out.String(), "VERDICT: prior coverage found.") ||
		errb.String() != "check-coverage: cannot read "+bad+": "+dirReason()+"\n" {
		t.Errorf("exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}
