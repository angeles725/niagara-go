package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyref"
)

// The two niagara-help trees the parity tests read: the Python reference
// (helpPyRef, the code) and the live checkout (niagaraHelpData, the data).
// The real-data parity tests read the live checkout as DATA only:
// versions/registry.json, the version roots it names and, next to the
// checkout, sources/n4-dev-course. The Python they compare against comes from
// the pinned reference, never from this checkout, so the data lookup keys on
// the registry and works once tools/niagara_help_lib is deleted.

// helpPyRefEnv names the niagara-help Python reference (internal/pyref).
const helpPyRefEnv = "NIAGARA_HELP_PY_REF"

// helpPyRef is the niagara-help Python reference worktree: a missing one is
// a missingReference (a failure when the run requested parity, even without
// PYREF_REQUIRED=1, and pyref.Dir's failure with it), an existing one must
// pass pyref.Dir's pin and clean-worktree check (once per directory and test
// binary: every parity test asks).
func helpPyRef(t *testing.T) string {
	t.Helper()
	dir := os.Getenv(helpPyRefEnv)
	if dir == "" {
		dir = pyref.DefaultDir("niagara-help")
	}
	if st, err := os.Stat(dir); (err != nil || !st.IsDir()) && os.Getenv(pyref.RequiredEnv) != "1" {
		missingReference(t, fmt.Sprintf("no niagara-help Python reference at %s (%s unset or wrong; create it with "+
			"git -C <niagara-help checkout> worktree add --detach %s %s): parity not checked",
			dir, helpPyRefEnv, pyref.DefaultDir("niagara-help"), pyref.Tag))
	}
	if _, ok := helpPyRefChecked.Load(dir); ok {
		return dir
	}
	ref := pyref.Dir(t, "niagara-help")
	helpPyRefChecked.Store(ref, true)
	return ref
}

// helpPyRefChecked holds the reference directories pyref.Dir accepted.
var helpPyRefChecked sync.Map

// helpDataMarker is the file that makes a folder a niagara-help data root.
var helpDataMarker = filepath.Join("versions", "registry.json")

// niagaraHelpData is the niagara-help data root: $NIAGARA_HELP_DIR when the
// variable is set; else $NIAGARA_RESEARCH/niagara-help when it holds the
// registry; else the first checkout met walking up from the test directory,
// at each level as <dir>/niagara-help or <dir>/niagara-research/niagara-help
// (a worktree under ~/niagara-go-worktrees finds
// ~/niagara-research/niagara-help). "" when there is none.
func niagaraHelpData() string {
	if env := os.Getenv("NIAGARA_HELP_DIR"); env != "" {
		return env
	}
	if env := os.Getenv("NIAGARA_RESEARCH"); env != "" {
		help := filepath.Join(env, "niagara-help")
		if isHelpData(help) {
			return help
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, sub := range []string{"niagara-help", filepath.Join("niagara-research", "niagara-help")} {
			if help := filepath.Join(dir, sub); isHelpData(help) {
				return help
			}
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

func isHelpData(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, helpDataMarker))
	return err == nil && st.Mode().IsRegular()
}

// The data-root lookup: $NIAGARA_HELP_DIR wins; $NIAGARA_RESEARCH is used
// when <it>/niagara-help holds the registry, else the walk up decides; a
// checkout holding only the Python (no registry) is not a data root.
func TestNiagaraHelpDataLookup(t *testing.T) {
	help := filepath.Join(t.TempDir(), "nh")
	t.Setenv("NIAGARA_HELP_DIR", help)
	t.Setenv("NIAGARA_RESEARCH", "")
	if got := niagaraHelpData(); got != help {
		t.Fatalf("NIAGARA_HELP_DIR: %q", got)
	}
	research := t.TempDir()
	writeFile(t, filepath.Join(research, "niagara-help", helpDataMarker), []byte("{}"))
	t.Setenv("NIAGARA_HELP_DIR", "")
	t.Setenv("NIAGARA_RESEARCH", research)
	if got := niagaraHelpData(); got != filepath.Join(research, "niagara-help") {
		t.Fatalf("NIAGARA_RESEARCH: %q", got)
	}
	codeOnly := t.TempDir()
	writeFile(t, filepath.Join(codeOnly, "niagara-help", "tools", "niagara_help_lib", "install_docs.json"), []byte("{}"))
	t.Setenv("NIAGARA_RESEARCH", codeOnly)
	if got := niagaraHelpData(); strings.HasPrefix(got, codeOnly) {
		t.Fatalf("NIAGARA_RESEARCH with the Python but no registry: %q", got)
	}
}

// A plain run without the niagara-help Python reference skips the parity
// tests (helpPyRef), as it does without python3.
func TestHelpPyRefMissingSkips(t *testing.T) {
	for _, v := range parityVars {
		t.Setenv(v, "")
	}
	t.Setenv("PYREF_REQUIRED", "")
	t.Setenv(helpPyRefEnv, filepath.Join(t.TempDir(), "no-ref"))
	var skipped bool
	t.Run("ref", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		helpPyRef(t)
	})
	if !skipped {
		t.Error("a missing reference in a plain run did not skip")
	}
}
