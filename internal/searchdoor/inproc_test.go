package searchdoor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// The in-process answers are compared with what the PYTHON backends printed on
// the fixture checkouts the Go ports already freeze (corpus-nav, niagara_help and
// module_nav goldens): rc, stdout and stderr must be those bytes, and every case
// only exec can answer exactly must say so (ok false).

// oracle is one recorded Python run.
type oracle struct {
	rc             int
	stdout, stderr string
}

func readOracle(t *testing.T, dir string) oracle {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	rc, err := strconv.Atoi(strings.TrimSpace(read("exit")))
	if err != nil {
		t.Fatalf("%s/exit: %v", dir, err)
	}
	return oracle{rc, read("stdout"), read("stderr")}
}

func checkAnswer(t *testing.T, got answer, want oracle) {
	t.Helper()
	if !got.ok || got.rc != want.rc || got.stdout != want.stdout || got.stderr != want.stderr {
		t.Errorf("ok %v exit %d (want %d) stderr %q (want %q) stdout:\n%s\nwant:\n%s",
			got.ok, got.rc, want.rc, got.stderr, want.stderr, got.stdout, want.stdout)
	}
}

// stubScript creates the backend script the in-process path requires to exist.
func stubScript(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# stub: answered in process\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBlocksFindMatchesCorpusNav(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	t.Setenv("CORPUS_NAV_ROOT", "")
	t.Setenv("CORPUS_NAV_INDEX_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../corpusnav/testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	script := stubScript(t, filepath.Join(root, "tools", "corpus-nav.py"))
	for golden, args := range map[string][]string{
		"find-alarm-histories-and":           {"find", "alarm histories", "--limit", "50", "--and"},
		"find-escalacion-de-alarmas-limit-3": {"find", "escalacion de alarmas", "--limit", "3"},
		"find-zzqq":                          {"find", "zzqq", "--limit", "2000", "--or"},
	} {
		t.Run(golden, func(t *testing.T) {
			checkAnswer(t, blocksFind(append([]string{script}, args...)),
				readOracle(t, filepath.Join("..", "corpusnav", "testdata", "golden", golden)))
		})
	}
	for _, args := range [][]string{{"find", "-x", "--limit", "50"}, {"find", "alarm", "--limit", "0"},
		{"find", "alarm", "--in", "all"}, {"grep", "alarm", "--limit", "50"}} {
		if blocksFind(append([]string{script}, args...)).ok {
			t.Errorf("%v: answered in process, want exec", args)
		}
	}
	if blocksFind([]string{filepath.Join(t.TempDir(), "tools", "corpus-nav.py"), "find", "a", "--limit", "5"}).ok {
		t.Error("a folder without CATALOG.md: answered in process, want exec")
	}
}

func TestDocsFindMatchesNiagaraHelp(t *testing.T) {
	testdata, err := filepath.Abs("../help/testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("NIAGARA_HELP_DIR", "")
	checkout := ftsfixture.Checkout(t, testdata, false)
	script := stubScript(t, filepath.Join(checkout, "tools", "niagara_help.py"))
	raw, err := os.ReadFile(filepath.Join(testdata, "fulltext-cli", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string   `json:"name"`
		Args []string `json:"args"`
		Exit int      `json:"exit"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cases {
		if !strings.HasPrefix(c.Name, "find") {
			continue
		}
		n++
		t.Run(c.Name, func(t *testing.T) {
			got := docsFind(append([]string{script}, docsArgv(c.Args)...))
			if c.Exit != 0 { // a miss or an unsearchable query: niagara-help's own block, via exec
				if got.ok {
					t.Errorf("exit %d case answered in process", c.Exit)
				}
				return
			}
			want, err := os.ReadFile(filepath.Join(testdata, "fulltext-cli", c.Name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			checkAnswer(t, got, oracle{0, strings.ReplaceAll(string(want), ftsfixture.Root, checkout), ""})
		})
	}
	if n < 6 {
		t.Fatalf("only %d find cases in cases.json", n)
	}
	stale := ftsfixture.Checkout(t, testdata, true) // D21: no index Go may read
	if docsFind([]string{stubScript(t, filepath.Join(stale, "tools", "niagara_help.py")), "find", "alarm", "-n", "5"}).ok {
		t.Error("without fulltext.sqlite: answered in process, want exec")
	}
	if docsFind([]string{script, "--niagara-version", "9.9", "find", "alarm", "-n", "5"}).ok {
		t.Error("unknown version: answered in process, want exec")
	}
}

// docsArgv is a recorded niagara_help command line in docsJob's order:
// [--niagara-version V] find <query> -n N [--and|--or].
func docsArgv(args []string) []string {
	var head, flag []string
	n, query := "5", ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--niagara-version", "-n":
			if a == "-n" {
				n = args[i+1]
			} else {
				head = append(head, a, args[i+1])
			}
			i++
		case "--and", "--or":
			flag = []string{a}
		case "find":
		default:
			query = a
		}
	}
	return append(append(head, "find", query, "-n", n), flag...)
}

func TestCodeTokenAndGrepMatchModuleNav(t *testing.T) {
	organized, err := filepath.Abs("../nav/testdata/fixture/search-organized")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAV_CORPUS_BASE", organized)
	base := filepath.Join(t.TempDir(), "module-navigator")
	if err := os.CopyFS(filepath.Join(base, "indexes"), os.DirFS("../nav/testdata/fixture/search/indexes")); err != nil {
		t.Fatal(err)
	}
	script := stubScript(t, filepath.Join(base, "tools", "module_nav.py"))
	for golden, args := range map[string][]string{
		"nav-token":          {"token", "ackAlarm", "-n", "100000"},
		"nav-token-limit":    {"token", "ackAlarm", "-n", "4"},
		"nav-token-miss":     {"token", "zzNothing", "-n", "100000"},
		"nav-token-similar":  {"token", "ackalar", "-n", "100000"},
		"nav-token-tie":      {"token", "Entry", "-n", "100000"},
		"nav-grep":           {"grep", "ackAlarm", "-n", "3"},
		"nav-grep-all":       {"grep", "ackAlarm|Ñandú", "-n", "30"},
		"nav-grep-long-line": {"grep", "veryLong", "-n", "1"},
	} {
		t.Run(golden, func(t *testing.T) {
			run := codeToken
			if args[0] == "grep" {
				run = codeGrep
			}
			checkAnswer(t, run(append([]string{script}, args...)),
				readOracle(t, filepath.Join("..", "nav", "testdata", "golden", golden)))
		})
	}
	if codeGrep([]string{script, "grep", "ack(Alarm", "-n", "30"}).ok {
		t.Error("invalid regex: answered in process, want exec")
	}
	if codeToken([]string{script, "token", "ackAlarm", "-m", "alarm-rt"}).ok {
		t.Error("a token filter nsearch never sends: answered in process, want exec")
	}
	t.Setenv("NAV_CORPUS_BASE", filepath.Join(t.TempDir(), "absent"))
	if codeGrep([]string{script, "grep", "ackAlarm", "-n", "30"}).ok {
		t.Error("no corpus: answered in process, want exec")
	}
}

// recordingBackend is the exec stand-in: it records each call it is handed.
type recordingBackend struct{ calls [][]string }

func (r *recordingBackend) Run(argv []string, _ float64) Raw {
	r.calls = append(r.calls, argv)
	return Raw{State: Empty, Out: "(no matches; exec)", RC: 1, HasRC: true}
}

// TestInprocRoutesToExec: calls with no in-process answer (another command, a
// missing script, a path that is not one of the sources) reach exec unchanged.
func TestInprocRoutesToExec(t *testing.T) {
	dir := t.TempDir()
	code := stubScript(t, filepath.Join(dir, "tools", "module_nav.py"))
	rec := &recordingBackend{}
	b := &inprocBackend{exec: rec, clock: time.Now,
		paths: Paths{Blocks: filepath.Join(dir, "absent.py"), Docs: filepath.Join(dir, "nh.py"), Code: code}}
	for _, argv := range [][]string{
		{code, "rank", "-n", "20", "--", "alarm"},
		{code, "token", "BAlarm", "-n", "100000"}, // no indexes/ beside tools/
		{b.paths.Blocks, "find", "alarm", "--limit", "2000"},
		{filepath.Join(dir, "other.py"), "find", "alarm"},
	} {
		if raw := b.Run(argv, 5); raw.Out != "(no matches; exec)" {
			t.Errorf("%v: %+v", argv, raw)
		}
	}
	if len(rec.calls) != 4 {
		t.Errorf("exec calls %d, want 4", len(rec.calls))
	}
}

func TestBoundedTimesOut(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	if _, done := bounded(func() answer { <-release; return exact(0, "late", "") }, 0.01); done {
		t.Error("a run past the timeout reported done")
	}
	if a, done := bounded(func() answer { return exact(0, "x", "") }, 5); !done || a.stdout != "x" {
		t.Errorf("a quick run: %v %+v", done, a)
	}
}

// A library panic in process must stay that source's failure, as a crashed
// backend process was: bounded hands it to the caller's goroutine, where the
// source guard types it UNAVAILABLE.
func TestBoundedPanicReachesTheSourceGuard(t *testing.T) {
	o := guarded(job{"docs", "SOURCE 2", "niagara_help find", func() *Outcome {
		bounded(func() answer { panic("corrupt cache") }, 5)
		return nil
	}})
	if o.State != Unavailable ||
		o.Reason != "unavailable: the front door failed on this source: panic: corrupt cache" {
		t.Errorf("got %+v", o)
	}
}

// A run that panics after its answer was dropped must not take the process down.
func TestBoundedPanicPastTimeoutIsDropped(t *testing.T) {
	release, panicking := make(chan struct{}), make(chan struct{})
	if _, done := bounded(func() answer {
		<-release
		defer close(panicking)
		panic("late")
	}, 0.01); done {
		t.Fatal("a run past the timeout reported done")
	}
	close(release)
	<-panicking
	time.Sleep(50 * time.Millisecond) // an unrecovered panic would end the test binary here
}
