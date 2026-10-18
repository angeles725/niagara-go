package searchdoor

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/corpusnav"
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
			checkAnswer(t, blocksFind(os.Getenv, append([]string{script}, args...)),
				readOracle(t, filepath.Join("..", "corpusnav", "testdata", "golden", golden)))
		})
	}
	for _, args := range [][]string{{"find", "alarm", "--limit", "0"},
		{"find", "alarm", "--in", "all"}, {"grep", "alarm", "--limit", "50"}} {
		if blocksFind(os.Getenv, append([]string{script}, args...)).ok {
			t.Errorf("%v: answered in process, want exec", args)
		}
	}
	// A query that starts with "-" is searched as text (corpus-nav's argparse read it as an option).
	if a := blocksFind(os.Getenv, append([]string{script}, "find", "-zzqq", "--limit", "50")); !a.ok || a.rc != 1 ||
		!MissLine.MatchString(a.stdout) {
		t.Errorf("a query starting with '-': %+v", a)
	}
}

// TestBlocksFindWithoutCatalogMatchesCorpusNav: a folder without CATALOG.md is
// searched in process exactly as corpus-nav searches it.
func TestBlocksFindWithoutCatalogMatchesCorpusNav(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	t.Setenv("CORPUS_NAV_INDEX_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "niagara-mental-model-bloque7.md"),
		[]byte("# Block 7\n\nalarm routing notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := stubScript(t, filepath.Join(root, "tools", "corpus-nav.py"))
	for _, query := range []string{"alarm", "zzqq"} {
		args := []string{"find", query, "--limit", "2000"}
		t.Setenv("CORPUS_NAV_ROOT", "")
		got := blocksFind(os.Getenv, append([]string{script}, args...))
		t.Setenv("CORPUS_NAV_ROOT", root)
		var out, errb bytes.Buffer
		rc := corpusnav.Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		checkAnswer(t, got, oracle{rc, out.String(), errb.String()})
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
	t.Parallel()
	env := envOf(map[string]string{"NAV_CORPUS_BASE": organized})
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
				run = func(argv []string) answer { return codeGrep(env, argv) }
			}
			want := readOracle(t, filepath.Join("..", "nav", "testdata", "golden", golden))
			if golden == "nav-token-similar" { // tokenMiss leaves out the similar tokens (intended)
				similar := regexp.MustCompile(`(?m)^  Similar tokens: .*\n`)
				if !similar.MatchString(want.stdout) {
					t.Fatalf("the oracle has no similar-tokens line:\n%s", want.stdout)
				}
				want.stdout = similar.ReplaceAllString(want.stdout, "")
			}
			checkAnswer(t, run(append([]string{script}, args...)), want)
		})
	}
	if codeGrep(env, []string{script, "grep", "ack(Alarm", "-n", "30"}).ok {
		t.Error("invalid regex: answered by the fast path, want mnav's error")
	}
	if codeToken([]string{script, "token", "ackAlarm", "-m", "alarm-rt"}).ok {
		t.Error("a token filter nsearch never sends: answered in process, want no answer")
	}
	absent := envOf(map[string]string{"NAV_CORPUS_BASE": filepath.Join(t.TempDir(), "absent")})
	if codeGrep(absent, []string{script, "grep", "ackAlarm", "-n", "30"}).ok {
		t.Error("no corpus: answered in process, want no answer")
	}
}

// TestInprocUnknownCalls: a script path that is not a file is the Python door's
// "tool not found"; a call no source answers is typed UNAVAILABLE, never a zero.
func TestInprocUnknownCalls(t *testing.T) {
	dir := t.TempDir()
	code := stubScript(t, filepath.Join(dir, "tools", "module_nav.py"))
	blocks := stubScript(t, filepath.Join(dir, "tools", "corpus-nav.py"))
	b := &inprocBackend{clock: time.Now, paths: Paths{Blocks: blocks, Docs: filepath.Join(dir, "absent.py"), Code: code}}
	for _, c := range []struct {
		argv []string
		want string
	}{
		{[]string{b.paths.Docs, "find", "alarm", "-n", "5"}, "unavailable: tool not found at " + pyPath(b.paths.Docs)},
		{[]string{dir, "find", "alarm", "-n", "5"}, "unavailable: tool not found at " + pyPath(dir)},
		{[]string{code}, "unavailable: no in-process answer for `module_nav.py` (a call nsearch never makes)"},
		{[]string{stubScript(t, filepath.Join(dir, "other.py")), "x"},
			"unavailable: no in-process answer for `other.py x` (a call nsearch never makes)"},
		{[]string{blocks, "grep", "alarm", "--limit", "50"},
			"unavailable: no in-process answer for `corpus-nav.py grep alarm --limit 50` (a call nsearch never makes)"},
	} {
		if raw := b.Run(c.argv, 5); raw.State != Unavailable || raw.Reason != c.want || raw.HasRC {
			t.Errorf("%v: %+v, want %q", c.argv, raw, c.want)
		}
	}
	if raw := b.Run([]string{blocks, "find", "alarm", "--limit", "50"}, 0); raw.Reason != "unavailable: timed out after 0s" {
		t.Errorf("a zero budget: %+v", raw)
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

func TestCodeRankMatchesModuleNav(t *testing.T) {
	t.Setenv("NAV_CORPUS_BASE", filepath.Join(t.TempDir(), "no-corpus"))
	scripts := map[string]string{}
	for _, fixture := range []string{"nav", "rank-skips"} {
		base := filepath.Join(t.TempDir(), "module-navigator")
		if err := os.CopyFS(filepath.Join(base, "indexes"),
			os.DirFS(filepath.Join("..", "nav", "testdata", "fixture", fixture, "indexes"))); err != nil {
			t.Fatal(err)
		}
		scripts[fixture] = stubScript(t, filepath.Join(base, "tools", "module_nav.py"))
	}
	for golden, c := range map[string]struct{ fixture, n, question string }{
		"nav-rank":          {"nav", "3", "como funciona el ack de alarmas"},
		"nav-rank-exact":    {"nav", "4", "que es un AlarmService y un BAckState"},
		"nav-rank-accents":  {"nav", "2", "configuración de la consola"},
		"nav-rank-all":      {"nav", "20", "alarm"},
		"nav-rank-miss":     {"nav", "20", "zzzqqq xxyyzz"},
		"nav-rank-no-terms": {"nav", "20", "the a of"},
		"nav-rank-skips":    {"rank-skips", "20", "alarm service"},
	} {
		t.Run(golden, func(t *testing.T) {
			checkAnswer(t, codeRank([]string{scripts[c.fixture], "rank", "-n", c.n, "--", c.question}),
				readOracle(t, filepath.Join("..", "nav", "testdata", "golden", golden)))
		})
	}
	if codeRank([]string{scripts["nav"], "rank", "alarm"}).ok {
		t.Error("a rank line nsearch never sends: answered in process, want no answer")
	}
	if err := os.Remove(filepath.Join(filepath.Dir(filepath.Dir(scripts["nav"])), "indexes", "xref-index.json")); err != nil {
		t.Fatal(err)
	}
	if codeRank([]string{scripts["nav"], "rank", "-n", "20", "--", "alarm"}).ok {
		t.Error("absent xref index (module_nav prints a NOTE): answered in process, want no answer")
	}
}

// TestInprocBudget: a call past its timeout is typed timed out like a killed
// process.
func TestInprocBudget(t *testing.T) {
	script := stubScript(t, filepath.Join(t.TempDir(), "tools", "module_nav.py"))
	now := time.Unix(0, 0)
	release := make(chan struct{})
	defer close(release)
	slow := func(Paths, envFunc, []string) func([]string) answer {
		return func([]string) answer { <-release; return exact(0, "late", "") }
	}
	b := &inprocBackend{clock: func() time.Time { return now }, pick: slow}
	raw := b.Run([]string{script, "token", "BAlarm", "-n", "5"}, 0.01)
	if raw.State != Unavailable || raw.Reason != "unavailable: timed out after 0.01s" || raw.MS != 10 {
		t.Errorf("timed out: %+v", raw)
	}
}
