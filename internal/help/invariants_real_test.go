package help

// Corpus invariants carried over from niagara-help
// tests/test_corpus_invariants.py: architectural bedrock of the Baja
// component model (BComponent extends BComplex, an alarm ext implements
// BIAlarmSource, ...) checked on the real 4.14.0.162 root corpus named by
// NIAGARA_HELP_DIR. They break only when an index or a command genuinely
// regresses after a reindex; counts are loose lower bounds so a legitimate
// reindex that adds classes never fails them. The lookups run through nh
// (Main) where the command still exists; hierarchy and implementors are
// retired in the Go port (decision D32), so those invariants read
// inheritance.json directly.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// realVersion is the corpus the Python invariants and golden set ran on:
// the root data root of the niagara-help checkout.
const realVersion = "4.14.0.162"

// realCorpus is the checkout named by NIAGARA_HELP_DIR and the data root of
// realVersion in it. It skips when the variable is unset and fails when it
// is set but the checkout or that version is unusable.
func realCorpus(t *testing.T) (repo, base string) {
	t.Helper()
	repo = os.Getenv(EnvDir)
	if repo == "" {
		t.Skip("NIAGARA_HELP_DIR is not set: the real-corpus checks need the niagara-help main checkout")
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(repo, "indexes")); err != nil || !st.IsDir() {
		t.Fatalf("NIAGARA_HELP_DIR=%s has no indexes/ directory", repo)
	}
	_, base, err = scope.Resolve(repo, realVersion, func(string) string { return "" })
	if err != nil {
		t.Fatalf("NIAGARA_HELP_DIR=%s: version %s: %v", repo, realVersion, err)
	}
	return repo, base
}

// nhReal runs nh on realVersion of the real checkout and returns its stdout.
// Like the Python harness it fails the test when nh would not exit 0: an
// error (exit 2) or a lookup that matched nothing (exit 1).
func nhReal(t *testing.T, args ...string) string {
	t.Helper()
	repo, _ := realCorpus(t)
	t.Setenv(EnvDir, repo)
	t.Setenv(envVersionForTest, "")
	t.Setenv("COLUMNS", "100")
	var out, errb bytes.Buffer
	argv := append([]string{"--niagara-version", realVersion}, args...)
	if code := Main(argv, cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb}); code != 0 {
		t.Fatalf("nh %s: exit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, out.String(), errb.String())
	}
	return out.String()
}

func wantIn(t *testing.T, what, text string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(text, s) {
			t.Errorf("%s: missing %q", what, s)
		}
	}
}

// inhRecord is one inheritance.json value, decoded for the invariants.
type inhRecord struct {
	Extends    string   `json:"extends"`
	Implements []string `json:"implements"`
	Subclasses []string `json:"subclasses"`
}

func loadInheritance(t *testing.T) map[string]inhRecord {
	t.Helper()
	_, base := realCorpus(t)
	raw, err := os.ReadFile(filepath.Join(base, "indexes", "inheritance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inh map[string]inhRecord
	if err := json.Unmarshal(raw, &inh); err != nil {
		t.Fatalf("inheritance.json: %v", err)
	}
	return inh
}

// bare drops a generic suffix (escaped or not) and a package qualifier.
func bare(name string) string {
	name, _, _ = strings.Cut(name, "&lt;")
	name, _, _ = strings.Cut(name, "<")
	return name[strings.LastIndexByte(name, '.')+1:]
}

// TestInvariantRealIndexesLoad: every index is valid JSON with a sane
// minimum size. call-index.json is not checked: the Go port neither builds
// nor reads it (decision I3-5).
func TestInvariantRealIndexesLoad(t *testing.T) {
	_, base := realCorpus(t)
	for name, minimum := range map[string]int{
		"class-index.json":  2000,
		"inheritance.json":  2000,
		"method-index.json": 10000,
		"source-index.json": 2000,
		"xref-index.json":   2000,
		"slots-index.json":  500,
	} {
		raw, err := os.ReadFile(filepath.Join(base, "indexes", name))
		if err != nil {
			t.Errorf("missing index %s: %v", name, err)
			continue
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(data) < minimum {
			t.Errorf("%s shrank to %d entries (< %d)", name, len(data), minimum)
		}
	}
}

// TestInvariantRealClass: BComponent is the root of the Baja component model.
func TestInvariantRealClass(t *testing.T) {
	out := nhReal(t, "class", "BComponent")
	wantIn(t, "class BComponent", out, "PACKAGE: javax.baja.sys", "TYPE: class", "EXTENDS: BComplex", "BIPropertyContainer")
	wantIn(t, "class BINavNode", nhReal(t, "class", "BINavNode"), "TYPE: interface")
}

// TestInvariantRealInheritance: the full ancestry of BAlarmService runs
// through BComponent and BAbstractService (hierarchy --chain), and
// BAbstractService is a direct subclass of BComponent (hierarchy).
func TestInvariantRealInheritance(t *testing.T) {
	inh := loadInheritance(t)
	chain := []string{"BAlarmService"}
	for seen := map[string]bool{}; ; {
		cur := chain[0]
		e, ok := inh[cur]
		if !ok || e.Extends == "" || seen[cur] {
			break
		}
		seen[cur] = true
		chain = append([]string{bare(e.Extends)}, chain...)
	}
	if got := strings.Join(chain, " > "); !strings.Contains(got, "BComponent > BAbstractService > BAlarmService") {
		t.Errorf("BAlarmService ancestry %q lacks BComponent > BAbstractService > BAlarmService", got)
	}
	if !slices.Contains(inh["BComponent"].Subclasses, "BAbstractService") {
		t.Errorf("BComponent subclasses lack BAbstractService: %q", inh["BComponent"].Subclasses)
	}
	if bare(inh["BAbstractService"].Extends) != "BComponent" {
		t.Errorf("BAbstractService extends %q, want BComponent", inh["BAbstractService"].Extends)
	}
}

// TestInvariantRealImplementors: BAlarmSourceExt, core alarm infrastructure,
// implements BIAlarmSource (implementors BIAlarmSource).
func TestInvariantRealImplementors(t *testing.T) {
	var impl []string
	for name, e := range loadInheritance(t) {
		for _, i := range e.Implements {
			if bare(i) == "BIAlarmSource" {
				impl = append(impl, bare(name))
			}
		}
	}
	if !slices.Contains(impl, "BAlarmSourceExt") {
		t.Errorf("implementors of BIAlarmSource lack BAlarmSourceExt: %q", impl)
	}
}

// TestInvariantRealSlots: a known BAlarmService property, and the reverse
// search by a widely used status type.
func TestInvariantRealSlots(t *testing.T) {
	wantIn(t, "slots BAlarmService", nhReal(t, "slots", "BAlarmService"), "alarmDbConfig")
	wantIn(t, "slots --by-type BStatusNumeric", nhReal(t, "slots", "--by-type", "BStatusNumeric"),
		"slots of type 'BStatusNumeric'")
}

// TestInvariantRealFullText: find returns hits from more than one source,
// and stats reports the indexes.
func TestInvariantRealFullText(t *testing.T) {
	out := nhReal(t, "find", "alarm", "-n", "5")
	_, bySource, _ := strings.Cut(out, "\nby source: ")
	bySource, _, _ = strings.Cut(bySource, "\n")
	if n := len(strings.Split(bySource, ", ")); bySource == "" || n < 2 {
		t.Errorf("find alarm: hits from fewer than two sources:\n%s", out)
	}
	wantIn(t, "stats", nhReal(t, "stats"), "class-index.json", "INDEXES")
}
