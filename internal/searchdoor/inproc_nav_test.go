package searchdoor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav"
)

// navCheckout lays a module-navigator fixture out as the real checkout is:
// <root>/module-navigator/{indexes,tools/module_nav.py} next to <root>/organized.
// It returns the script and the organized/ folder.
func navCheckout(t *testing.T, indexes, organized string) (script, corpus string) {
	t.Helper()
	root := t.TempDir()
	fixture := filepath.Join("..", "nav", "testdata", "fixture")
	checkout := filepath.Join(root, "module-navigator")
	corpus = filepath.Join(root, "organized")
	for src, dst := range map[string]string{filepath.Join(fixture, indexes, "indexes"): filepath.Join(checkout, "indexes"),
		filepath.Join(fixture, organized): corpus} {
		if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
			t.Fatal(err)
		}
	}
	return stubScript(t, filepath.Join(checkout, "tools", "module_nav.py")), corpus
}

// navOracle is mnav run on the same checkout with explicit paths.
func navOracle(t *testing.T, script, corpus string, args []string) Raw {
	t.Helper()
	t.Setenv("NAV_CORPUS_BASE", corpus)
	defer t.Setenv("NAV_CORPUS_BASE", "")
	var out, errb bytes.Buffer
	rc := nav.Main(append([]string{"--base-dir", filepath.Dir(filepath.Dir(script))}, args...),
		cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return Classify(rc, out.String(), errb.String(), 0)
}

// TestCodeSourceNeverExecs: every module_nav call the door makes (the structure
// sub-scope, the drivers probe, the license join, and the token/grep/rank calls
// the fast paths leave) is answered in process with mnav's own bytes; exec is
// never reached.
func TestCodeSourceNeverExecs(t *testing.T) {
	t.Setenv("NAV_CORPUS_BASE", "")
	t.Setenv("MODULE_NAVIGATOR_DIR", "")
	for _, fx := range []struct {
		indexes, organized string
		calls              [][]string
	}{
		{"nav", "organized", [][]string{
			{"hierarchy", "BAlarmRecipient", "--depth", "3"}, {"hierarchy", "BAlarmServce", "--depth", "3"},
			{"implementors", "BIAlarmSource", "-n", "200"}, {"implementors", "BIAlarmSorce", "-n", "200"},
			{"modules"}, {"token", "BAlarmRecipient", "-n", "100000"}, // no token index: mnav's error
			{"rank", "-n", "20", "--", "alarm recipient"}, // no xref index: mnav's NOTE
		}},
		{"drivers", "drivers-organized", [][]string{
			{"drivers"}, {"drivers", "acme-rt"}, {"drivers", "alarm-rt"}, {"drivers", "--help"},
		}},
		{"lic", "lic-organized", [][]string{
			{"license-features", "--module", "lic-rt"}, {"license-features", "--module", "nope-rt"},
			{"license-feature", "LICFEAT"}, {"license-feature", "NOPE"},
		}},
		{"search", "search-organized", [][]string{{"grep", "(unclosed", "-n", "30"}}},
	} {
		script, corpus := navCheckout(t, fx.indexes, fx.organized)
		rec := &recordingBackend{}
		b := &inprocBackend{exec: rec, clock: time.Now, paths: Paths{Code: script}}
		for _, args := range fx.calls {
			t.Run(fx.indexes+" "+strings.Join(args, " "), func(t *testing.T) {
				got := b.Run(append([]string{script}, args...), 30)
				want := navOracle(t, script, corpus, args)
				got.MS = 0
				if got != want {
					t.Errorf("got  %+v\nwant %+v", got, want)
				}
			})
		}
		if len(rec.calls) != 0 {
			t.Errorf("%s: exec reached for %v", fx.indexes, rec.calls)
		}
	}
}

// TestCodeSourceWithoutIndexes: a module_nav.py with no indexes/ beside its tools/
// folder is module_nav's "Cannot detect base directory" (exit 1), in process.
func TestCodeSourceWithoutIndexes(t *testing.T) {
	script := stubScript(t, filepath.Join(t.TempDir(), "tools", "module_nav.py"))
	rec := &recordingBackend{}
	b := &inprocBackend{exec: rec, clock: time.Now, paths: Paths{Code: script}}
	for _, args := range [][]string{{"rank", "-n", "20", "--", "alarm"}, {"token", "BAlarm", "-n", "100000"},
		{"hierarchy", "BAlarm", "--depth", "3"}} {
		raw := b.Run(append([]string{script}, args...), 5)
		if raw.State != Unavailable || raw.RC != 1 || !strings.HasPrefix(raw.Out, "ERROR: Cannot detect base directory.") {
			t.Errorf("%v: %+v", args, raw)
		}
	}
	if len(rec.calls) != 0 {
		t.Errorf("exec reached for %v", rec.calls)
	}
}
