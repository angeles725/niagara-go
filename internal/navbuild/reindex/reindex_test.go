package reindex

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func TestIEC(t *testing.T) {
	// Pinned against GNU coreutils `numfmt --to=iec`.
	for n, want := range map[int64]string{
		0: "0", 1023: "1023", 1024: "1.0K", 1025: "1.1K", 1536: "1.5K", 10239: "10K", 10240: "10K",
		10752: "11K", 102400: "100K", 1048575: "1.0M", 1048576: "1.0M", 1100000: "1.1M", 365332: "357K",
		3740761: "3.6M", 34144665: "33M", 1006706688: "961M", 10485760: "10M", 10485761: "11M",
	} {
		if got := IEC(n); got != want {
			t.Errorf("IEC(%d) = %q, want %q", n, got, want)
		}
	}
}

var ansi = regexp.MustCompile("\033\\[[0-9]+m")

// reindex runs Main and returns its exit code and its stdout without colours.
func reindex(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return code, ansi.ReplaceAllString(out.String(), ""), errb.String()
}

// fixture is a module-navigator dir with an empty indexes/ and the synthetic
// corpus of internal/navbuild.
func fixture(t *testing.T) (nav, organized string) {
	t.Helper()
	nav = t.TempDir()
	if err := os.Mkdir(filepath.Join(nav, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	organized, err := filepath.Abs(filepath.Join("..", "testdata", "organized"))
	if err != nil {
		t.Fatal(err)
	}
	return nav, organized
}

// lenient lets the synthetic corpus pass the size checks (its indexes are
// all under 10 KiB) for the duration of a test.
func lenient(t *testing.T) {
	t.Helper()
	oldMin, oldShrink := minSize, shrinkMinPrev
	minSize, shrinkMinPrev = 1, 1
	t.Cleanup(func() { minSize, shrinkMinPrev = oldMin, oldShrink })
}

func TestReindexBuildsEveryKeptIndex(t *testing.T) {
	lenient(t)
	nav, organized := fixture(t)
	code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized)
	if code != cli.ExitFound || !strings.Contains(out, "Re-index complete") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, b := range Builders {
		if !isRegular(filepath.Join(nav, "indexes", b.Index)) {
			t.Errorf("%s not built", b.Index)
		}
		if !strings.Contains(out, "✓ "+b.Name+" (") {
			t.Errorf("no ✓ line for %s:\n%s", b.Name, out)
		}
		if !isRegular(filepath.Join(nav, "indexes", "logs", b.Name+".log")) {
			t.Errorf("no log for %s", b.Name)
		}
	}
	// A second run backs the first one's indexes up.
	if code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized); code != cli.ExitFound {
		t.Fatalf("second run: exit %d:\n%s", code, out)
	}
	for _, b := range Builders {
		if !isRegular(filepath.Join(nav, "indexes", ".reindex-bak", b.Index)) {
			t.Errorf("%s not backed up", b.Index)
		}
	}
}

func TestReindexTinyInventoryAbortsAndRollsBack(t *testing.T) {
	nav, organized := fixture(t)
	inv := filepath.Join(nav, "indexes", "module-inventory.json")
	old := []byte(`{"_meta": {}, "modules": {}}`)
	if err := os.WriteFile(inv, old, 0o644); err != nil {
		t.Fatal(err)
	}
	// The synthetic inventory is under 10 KiB: "tiny" with the default checks.
	code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized)
	if code != cli.ExitNoMatch {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{"✗ build_module_inventory.py (", "tiny (", "rolled back module-inventory.json from backup",
		"ABORT: build_module_inventory.py failed; dependent builders not run."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if got, _ := os.ReadFile(inv); !bytes.Equal(got, old) {
		t.Errorf("inventory not rolled back: %q", got)
	}
	if isRegular(filepath.Join(nav, "indexes", "class-index.json")) {
		t.Error("the class index was built after the inventory failed")
	}
}

func TestReindexFailedBuilderRollsBackAndContinues(t *testing.T) {
	lenient(t)
	nav, organized := fixture(t)
	if code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized, "--no-backup"); code != cli.ExitFound {
		t.Fatalf("setup: exit %d:\n%s", code, out)
	}
	idx := filepath.Join(nav, "indexes")
	inh, _ := os.ReadFile(filepath.Join(idx, "inheritance.json"))
	if err := os.WriteFile(filepath.Join(idx, "class-index.json"), []byte(`{"classes": `), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized, "--only", "build_inheritance.py")
	if code != cli.ExitNoMatch || !strings.Contains(out, "✗ build_inheritance.py (") || !strings.Contains(out, "exit=1") ||
		!strings.Contains(out, "Re-index finished WITH ERRORS") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if got, _ := os.ReadFile(filepath.Join(idx, "inheritance.json")); !bytes.Equal(got, inh) {
		t.Error("inheritance.json changed by a failed build")
	}
	if log, _ := os.ReadFile(filepath.Join(idx, "logs", "build_inheritance.py.log")); !strings.HasPrefix(string(log), "ERROR: ") {
		t.Errorf("log = %q", log)
	}
	if strings.Contains(out, "build_xref.py") {
		t.Errorf("--only ran another builder:\n%s", out)
	}
}

func TestReindexCheckBuildsNothing(t *testing.T) {
	nav, organized := fixture(t)
	code, out, _ := reindex(t, "--base-dir", nav, "--organized", organized, "--check")
	if code != cli.ExitFound || !strings.Contains(out, "Plan (dependency order):") ||
		!strings.Contains(out, "   5. build_token_index.py         -> token-index.db         (current: 0)") ||
		!strings.Contains(out, "Preflight OK. Run without --check to rebuild.") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if entries, _ := os.ReadDir(filepath.Join(nav, "indexes")); len(entries) != 0 {
		t.Errorf("--check wrote %d entries", len(entries))
	}
}

func TestReindexSetupErrors(t *testing.T) {
	nav, organized := fixture(t)
	t.Setenv("NAV_CORPUS_BASE", "")
	for _, tc := range []struct {
		name string
		args []string
		code int
		out  string // stdout, else stderr
	}{
		{"unknown arg", []string{"--fast"}, cli.ExitCannotSearch, "Unknown arg: --fast"},
		{"--only without a name", []string{"--only"}, cli.ExitCannotSearch, "--only needs a builder name"},
		{"--organized not a dir", []string{"--organized", filepath.Join(nav, "nope")}, cli.ExitCannotSearch, "--organized: not a directory"},
		{"unknown builder", []string{"--base-dir", nav, "--organized", organized, "--only", "build_string_index.py"},
			cli.ExitCannotSearch, "--only: unknown builder 'build_string_index.py'"},
		{"no indexes/", []string{"--base-dir", t.TempDir()}, cli.ExitNoMatch, "indexes/ not found at "},
		// the default corpus is organized/ next to the checkout
		{"no corpus", []string{"--base-dir", nav}, cli.ExitNoMatch, "Corpus dir not found: '" + filepath.Join(filepath.Dir(nav), "organized") + "'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := reindex(t, tc.args...)
			if code != tc.code || !strings.Contains(out+errs, tc.out) {
				t.Errorf("exit %d, want %d; output lacks %q:\n%s%s", code, tc.code, tc.out, out, errs)
			}
		})
	}
}
