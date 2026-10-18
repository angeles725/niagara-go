package audits

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func TestLexiconKeys(t *testing.T) {
	text := "# c\n a = 1\n\nb=2\x0bb=3\r\nnoequals\n=empty\n  a=4  c = x=y\n"
	want := []string{"a", "b", "b", "a", "c"}
	if got := lexiconKeys(text); !reflect.DeepEqual(got, want) {
		t.Errorf("lexiconKeys = %q, want %q", got, want)
	}
}

// runPalette runs one mnav command line ([--base-dir DIR] palette-lexicon-agents
// ...) the way nav.Main runs it, without importing nav (which imports this
// package): the global --base-dir lands in the namespace and is the runtime's
// base directory, which is nav (a checkout) when it is not given.
func runPalette(t *testing.T, nav string, args []string) (stdout, stderr string, exit int) {
	t.Helper()
	baseDir := ""
	if len(args) > 1 && args[0] == "--base-dir" {
		baseDir, nav, args = args[1], args[1], args[2:]
	}
	if len(args) == 0 || args[0] != "palette-lexicon-agents" {
		t.Fatalf("not a palette-lexicon-agents command line: %q", args)
	}
	p := argparse.New(argparse.Config{Prog: "module-nav palette-lexicon-agents"})
	argsPaletteLexiconAgents(p)
	var out, errb bytes.Buffer
	stdio := cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb}
	ns, code, done := p.Parse(args[1:], stdio)
	if !done {
		if baseDir != "" {
			ns.Set("base_dir", baseDir)
		}
		env := &registry.Env{Stdio: stdio, Runtime: core.NewAt("", nav, &out, &errb)}
		code = runPaletteLexiconAgents(env, ns)
	}
	return out.String(), errb.String(), code
}

// TestPaletteLexiconAgentsCases replays testdata/palette/golden: what
// niagara-research tools/palette-lexicon-agents.py printed for each command
// line over the synthetic trees next to it (gen_palette_lexicon.py).
func TestPaletteLexiconAgentsCases(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "palette"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := golden.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			stdout, stderr, exit := runPalette(t, "", c.Args)
			if exit != c.Exit {
				t.Errorf("exit = %d, want %d", exit, c.Exit)
			}
			if stdout != string(c.Stdout) {
				t.Errorf("stdout differs:\n%s", golden.Diff(c.Stdout, []byte(stdout)))
			}
			if stderr != string(c.Stderr) {
				t.Errorf("stderr differs:\n%s", golden.Diff(c.Stderr, []byte(stderr)))
			}
		})
	}
}

// TestPaletteLexiconAgentsNeedsModule is the script's p.error when neither a
// module nor --all is given (its usage line names the script, so it has no
// golden of its own).
func TestPaletteLexiconAgentsNeedsModule(t *testing.T) {
	base := t.TempDir()
	stdout, stderr, exit := runPalette(t, "", []string{"--base-dir", base, "palette-lexicon-agents", "--json"})
	if exit != 2 || stdout != "" {
		t.Errorf("exit %d, stdout %q; want 2 and nothing", exit, stdout)
	}
	want := "usage: module-nav palette-lexicon-agents [-h] [--all] [--json] [module]\n" +
		"module-nav palette-lexicon-agents: error: give a module name or --all\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	// A missing base directory is reported first, as the script does.
	_, stderr, exit = runPalette(t, "", []string{"--base-dir", base + "/x", "palette-lexicon-agents"})
	if exit != 2 || !strings.HasPrefix(stderr, "ERROR: base-dir not found: ") {
		t.Errorf("missing base: exit %d, stderr %q", exit, stderr)
	}
}

// TestPaletteLexiconAgentsDefaultBase: without the global --base-dir the
// command reads the organized/ corpus ($NAV_CORPUS_BASE here), as the script
// defaults to ../organized, not the module-navigator checkout.
func TestPaletteLexiconAgentsDefaultBase(t *testing.T) {
	tree, err := filepath.Abs(filepath.Join("testdata", "palette", "tree"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "palette", "golden", "multi", "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAV_CORPUS_BASE", tree)
	checkout := t.TempDir()
	stdout, stderr, exit := runPalette(t, checkout, []string{"palette-lexicon-agents", "multi"})
	if exit != 0 || stdout != string(want) || stderr != "" {
		t.Errorf("multi: exit %d, stderr %q, stdout differs:\n%s", exit, stderr, golden.Diff(want, []byte(stdout)))
	}
	_, stderr, exit = runPalette(t, checkout, []string{"palette-lexicon-agents", "nope"})
	if wantErr := "  ERROR: No artifacts for module 'nope'; searched " + tree + " (4 module dirs)\n"; exit != 1 || stderr != wantErr {
		t.Errorf("nope: exit %d, stderr %q, want 1 and %q", exit, stderr, wantErr)
	}
}
